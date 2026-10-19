# Owned URL runtime

This package implements the SDK's pinned WHATWG constructor parsing,
serialization, relative resolution, host canonicalization and origin contract.
It owns these algorithms and uses only the Go standard library plus the owned
Unicode 17 package. It contains no copied external engine source.

`URL` records are immutable after construction. Resolving a reference copies the
path and leaves the base unchanged. `Href`, hostname and origin use the same
parsed record. HTTP conversion to `net/url` happens only in the generated HTTP
adapter, after this package determines WHATWG semantics.

The parser includes special/non-special schemes, opaque paths/hosts, file drive
quirks, credentials, default ports, legacy IPv4 numbers, IPv6 canonicalization,
relative paths, dot segments and distinct empty query/fragment components. It
uses `net/netip` to validate IPv6, then serializes its pieces in URL form; zones
are rejected. The input boundary converts WTF8 lone surrogates to USVString
replacement characters and combines valid pairs.

IDNA uses nontransitional Unicode 17 mapping, NFC, Punycode, ContextJ and Bidi.
CheckHyphens/STD3/DNS-length checks are disabled as required for non-strict URL
parsing. ASCII domains take the standard's compatibility path, including invalid
ASCII Punycode spellings. URL-host forbidden characters are checked separately.
A small mark table avoids making IDNA require the complete regex property table.

The fixed corpora cover constructor parsing and UTF8 encoding. Setter objects,
DOM resolution, URLSearchParams and non-UTF8 HTML encoding arguments are outside
the SDK contract. Tests preserve omitted WPT origin expectations as omitted.
Seven pinned WPT IDNA expectations accept U+3F8CD or U+3E8AC, which the pinned
Unicode 17 mapping explicitly disallows. Original fixtures remain untouched;
`unicode17-overrides.json` records exact source/input hashes and rejection
expectations. Official Unicode 17 IDNA tests independently verify the policy.

References:

- [Pinned URL specification](https://github.com/whatwg/url/blob/fde3f74f063341a28d437216f75735b8b40128f5/url.bs)
- [UTS46 version35](https://www.unicode.org/reports/tr46/tr46-35.html)
- [Punycode RFC3492](https://www.rfc-editor.org/rfc/rfc3492)
- [ContextJ RFC5892](https://www.rfc-editor.org/rfc/rfc5892#appendix-A)
- [Bidi RFC5893](https://www.rfc-editor.org/rfc/rfc5893#section-2)
