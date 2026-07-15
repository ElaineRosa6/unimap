package service

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
	tcpFlagFIN = 0x01
	tcpFlagRST = 0x04
	tcpFlagPSH = 0x08
	tcpFlagURG = 0x20
)

func rawTCPFlags(method PortScanMethod) (byte, error) {
	switch method {
	case PortScanMethodFIN:
		return tcpFlagFIN, nil
	case PortScanMethodNULL:
		return 0, nil
	case PortScanMethodXmas:
		return tcpFlagFIN | tcpFlagPSH | tcpFlagURG, nil
	default:
		return 0, fmt.Errorf("%q is not a raw TCP scan method", method)
	}
}

func probeRawTCPPort(ctx context.Context, ipText string, port int, method PortScanMethod, timeout time.Duration) (string, string, error) {
	destinationIP := net.ParseIP(ipText).To4()
	if destinationIP == nil {
		return "", "", fmt.Errorf("raw TCP scan requires IPv4 target, got %q", ipText)
	}
	flags, err := rawTCPFlags(method)
	if err != nil {
		return "", "", err
	}
	sourceIP, err := sourceIPv4For(destinationIP, port)
	if err != nil {
		return "", "", fmt.Errorf("determine source address for raw TCP scan: %w", err)
	}
	packetConn, err := net.ListenPacket("ip4:tcp", sourceIP.String())
	if err != nil {
		return "", "", fmt.Errorf("open raw TCP socket (administrator/root or CAP_NET_RAW required): %w", err)
	}
	defer packetConn.Close()

	sourcePort := randomRawSourcePort()
	segment := buildTCPProbeSegment(sourceIP, destinationIP, sourcePort, port, flags)
	deadline := time.Now().Add(timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	_ = packetConn.SetDeadline(deadline)
	if _, err = packetConn.WriteTo(segment, &net.IPAddr{IP: destinationIP}); err != nil {
		return "", "", fmt.Errorf("send raw TCP %s probe: %w", method, err)
	}

	buffer := make([]byte, 2048)
	for {
		n, _, readErr := packetConn.ReadFrom(buffer)
		if readErr != nil {
			if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
				return "open_filtered", "no RST received", nil
			}
			return "", "", fmt.Errorf("receive raw TCP %s response: %w", method, readErr)
		}
		response := tcpPayload(buffer[:n])
		if len(response) < 20 || int(binary.BigEndian.Uint16(response[0:2])) != port || binary.BigEndian.Uint16(response[2:4]) != sourcePort {
			continue
		}
		if response[13]&tcpFlagRST != 0 {
			return "closed", "RST received", nil
		}
		return "open", "non-RST TCP response received", nil
	}
}

func sourceIPv4For(destination net.IP, port int) (net.IP, error) {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: destination, Port: port})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.To4(), nil
}

func randomRawSourcePort() uint16 {
	var value [2]byte
	if _, err := cryptorand.Read(value[:]); err != nil {
		return uint16(40000 + time.Now().UnixNano()%20000)
	}
	return 32768 + binary.BigEndian.Uint16(value[:])%28232
}

func buildTCPProbeSegment(sourceIP, destinationIP net.IP, sourcePort uint16, destinationPort int, flags byte) []byte {
	segment := make([]byte, 20)
	binary.BigEndian.PutUint16(segment[0:2], sourcePort)
	binary.BigEndian.PutUint16(segment[2:4], uint16(destinationPort))
	var sequence [4]byte
	_, _ = cryptorand.Read(sequence[:])
	copy(segment[4:8], sequence[:])
	segment[12] = 5 << 4
	segment[13] = flags
	binary.BigEndian.PutUint16(segment[14:16], 1024)
	binary.BigEndian.PutUint16(segment[16:18], tcpChecksum(sourceIP.To4(), destinationIP.To4(), segment))
	return segment
}

func tcpChecksum(sourceIP, destinationIP net.IP, segment []byte) uint16 {
	pseudoHeader := make([]byte, 12+len(segment))
	copy(pseudoHeader[0:4], sourceIP)
	copy(pseudoHeader[4:8], destinationIP)
	pseudoHeader[9] = 6
	binary.BigEndian.PutUint16(pseudoHeader[10:12], uint16(len(segment)))
	copy(pseudoHeader[12:], segment)
	var sum uint32
	for i := 0; i+1 < len(pseudoHeader); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(pseudoHeader[i : i+2]))
	}
	if len(pseudoHeader)%2 != 0 {
		sum += uint32(pseudoHeader[len(pseudoHeader)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func tcpPayload(packet []byte) []byte {
	if len(packet) >= 20 && packet[0]>>4 == 4 {
		headerLength := int(packet[0]&0x0f) * 4
		if headerLength >= 20 && headerLength <= len(packet) {
			return packet[headerLength:]
		}
	}
	return packet
}
