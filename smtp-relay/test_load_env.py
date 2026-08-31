#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Unit tests for smtp-relay .env loading. No network, no secrets."""
import os
import tempfile
import unittest

import relay


class LoadEnvFileTest(unittest.TestCase):
    def test_loads_unset_keys_and_skips_comments(self):
        fd, path = tempfile.mkstemp(suffix=".env")
        os.close(fd)
        try:
            with open(path, "w", encoding="utf-8") as fh:
                fh.write("# comment\nSMTP_USER=a@qq.com\nMAIL_TO=b@qq.com,c@qq.com\nEMPTY=\n")
            env = {}
            relay.load_env_file(path, env)
            self.assertEqual(env["SMTP_USER"], "a@qq.com")
            self.assertEqual(env["MAIL_TO"], "b@qq.com,c@qq.com")
            self.assertNotIn("EMPTY", env)
        finally:
            os.remove(path)

    def test_does_not_override_existing_environ(self):
        fd, path = tempfile.mkstemp(suffix=".env")
        os.close(fd)
        try:
            with open(path, "w", encoding="utf-8") as fh:
                fh.write("SMTP_USER=from-file@qq.com\n")
            env = {"SMTP_USER": "already-set@qq.com"}
            relay.load_env_file(path, env)
            self.assertEqual(env["SMTP_USER"], "already-set@qq.com")
        finally:
            os.remove(path)

    def test_missing_file_is_ok(self):
        env = {}
        relay.load_env_file(os.path.join(tempfile.gettempdir(), "no-such-unimap.env"), env)
        self.assertEqual(env, {})


if __name__ == "__main__":
    unittest.main()
