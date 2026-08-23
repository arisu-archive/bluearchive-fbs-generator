#!/usr/bin/env python3
"""Verify the shared table-key contract independently of the Go implementation."""

from __future__ import annotations

import base64
import json
import struct
from pathlib import Path

MASK32 = 0xFFFFFFFF
PRIME1 = 0x9E3779B1
PRIME2 = 0x85EBCA77
PRIME3 = 0xC2B2AE3D
PRIME4 = 0x27D4EB2F
PRIME5 = 0x165667B1


def rotate_left(value: int, count: int) -> int:
    return ((value << count) | (value >> (32 - count))) & MASK32


def xxhash32(data: bytes) -> int:
    offset = 0
    if len(data) >= 16:
        accumulators = [
            (PRIME1 + PRIME2) & MASK32,
            PRIME2,
            0,
            (-PRIME1) & MASK32,
        ]
        while offset + 16 <= len(data):
            for index in range(4):
                value = int.from_bytes(data[offset + index * 4 : offset + index * 4 + 4], "little")
                accumulator = (accumulators[index] + value * PRIME2) & MASK32
                accumulators[index] = (rotate_left(accumulator, 13) * PRIME1) & MASK32
            offset += 16
        digest = sum(
            rotate_left(accumulator, rotation)
            for accumulator, rotation in zip(accumulators, (1, 7, 12, 18), strict=True)
        ) & MASK32
    else:
        digest = PRIME5

    digest = (digest + len(data)) & MASK32
    while offset + 4 <= len(data):
        value = int.from_bytes(data[offset : offset + 4], "little")
        digest = (digest + value * PRIME3) & MASK32
        digest = (rotate_left(digest, 17) * PRIME4) & MASK32
        offset += 4
    while offset < len(data):
        digest = (digest + data[offset] * PRIME5) & MASK32
        digest = (rotate_left(digest, 11) * PRIME1) & MASK32
        offset += 1

    digest ^= digest >> 15
    digest = (digest * PRIME2) & MASK32
    digest ^= digest >> 13
    digest = (digest * PRIME3) & MASK32
    return (digest ^ (digest >> 16)) & MASK32


class MT19937:
    def __init__(self, seed: int) -> None:
        self.state = [0] * 624
        self.state[0] = seed & MASK32
        for index in range(1, 624):
            previous = self.state[index - 1]
            self.state[index] = (1812433253 * (previous ^ (previous >> 30)) + index) & MASK32
        self.index = 0

    def uint32(self) -> int:
        if self.index == 0:
            self._twist()
        value = self.state[self.index]
        value ^= value >> 11
        value ^= (value << 7) & 0x9D2C5680
        value ^= (value << 15) & 0xEFC60000
        value ^= value >> 18
        self.index = (self.index + 1) % 624
        return value & MASK32

    def bytes(self, length: int) -> bytes:
        output = bytearray()
        while len(output) < length:
            output.extend((self.uint32() >> 1).to_bytes(4, "little"))
        return bytes(output[:length])

    def _twist(self) -> None:
        for index in range(624):
            value = (self.state[index] & 0x80000000) | (self.state[(index + 1) % 624] & 0x7FFFFFFF)
            self.state[index] = self.state[(index + 397) % 624] ^ (value >> 1)
            if value & 1:
                self.state[index] ^= 0x9908B0DF


def table_key(table: str) -> tuple[int, bytes]:
    digest = xxhash32(table.encode("utf-8"))
    return digest, MT19937(digest).bytes(8)


def xor_with_key(value: bytes, key: bytes) -> bytes:
    return bytes(byte ^ key[index % len(key)] for index, byte in enumerate(value))


def verify() -> None:
    vectors = json.loads(Path(__file__).with_name("table_key_vectors.json").read_text(encoding="utf-8"))
    assert vectors["schema_version"] == 1
    assert vectors["contract"] == {
        "key_scope": "dto_tree",
        "root_default": "derive_from_root_table_name",
        "explicit_root_key": "preserve",
        "nested_key": "inherit_parent",
        "standalone_key": "derive_from_own_table_name",
        "without_decryption": "no_key_initialization_or_propagation",
        "whole_buffer_xor": "separate_preprocessing_step",
    }

    for vector in vectors["key_derivation"]:
        digest, key = table_key(vector["table"])
        assert f"{digest:08x}" == vector["xxhash32_hex"], vector["table"]
        assert key.hex() == vector["table_key_hex"], vector["table"]

    conversion = vectors["field_conversion"]
    key = bytes.fromhex(conversion["table_key_hex"])
    wrong_key = bytes.fromhex(conversion["wrong_child_key_hex"])

    int_vector = conversion["int32"]
    encoded_int = xor_with_key(struct.pack("<i", int_vector["plaintext_signed"]), key)
    assert struct.unpack("<i", encoded_int)[0] == int_vector["encoded_signed"]
    assert struct.unpack("<i", xor_with_key(encoded_int, wrong_key))[0] == int_vector["wrong_child_decode_signed"]

    string_vector = conversion["string"]
    plaintext = string_vector["plaintext"].encode("utf-16-le")
    assert plaintext.hex() == string_vector["plaintext_utf16le_hex"]
    encoded_string = xor_with_key(plaintext, key)
    assert encoded_string.hex() == string_vector["encoded_hex"]
    assert base64.b64encode(encoded_string).decode("ascii") == string_vector["encoded_base64"]


if __name__ == "__main__":
    verify()
    print("table-key vectors verified")
