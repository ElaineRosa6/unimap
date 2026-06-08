package tamper

// Tamper detection engine.
//
// Split into:
//   - detector_types.go   — types, constants, vars, constructors, storage operations
//   - detector_hash.go    — hash computation (ComputePageHash, segment hashes, etc.)
//   - detector_check.go   — tamper checking, batch operations, record storage
