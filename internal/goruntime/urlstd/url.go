// Package urlstd owns the SDK's WHATWG URL parsing and serialization contract.
package urlstd

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

var errURL = errors.New("invalid URL")

// URL is immutable after parsing. Accessors return serialized components.
type URL struct {
	scheme, username, password, hostname, port string
	path                                       []string
	opaque                                     string
	hasHost, opaquePath                        bool
	query, fragment                            string
	hasQuery, hasFragment                      bool
}

func (u *URL) Protocol() string { return u.scheme + ":" }
func (u *URL) Username() string { return u.username }
func (u *URL) Password() string { return u.password }
func (u *URL) Hostname() string { return u.hostname }
func (u *URL) Port() string     { return u.port }
func (u *URL) Host() string {
	if u.port != "" {
		return u.hostname + ":" + u.port
	}
	return u.hostname
}
func (u *URL) Pathname() string {
	if u.opaquePath {
		return u.opaque
	}
	if len(u.path) == 0 {
		return ""
	}
	return "/" + strings.Join(u.path, "/")
}
func (u *URL) Search() string {
	if u.query == "" {
		return ""
	}
	return "?" + u.query
}
func (u *URL) Hash() string {
	if u.fragment == "" {
		return ""
	}
	return "#" + u.fragment
}
func (u *URL) Href(excludeFragment bool) string {
	var out strings.Builder
	out.WriteString(u.Protocol())
	if u.hasHost {
		out.WriteString("//")
		if u.username != "" || u.password != "" {
			out.WriteString(u.username)
			if u.password != "" {
				out.WriteByte(':')
				out.WriteString(u.password)
			}
			out.WriteByte('@')
		}
		out.WriteString(u.Host())
	} else if !u.opaquePath && len(u.path) > 1 && u.path[0] == "" {
		out.WriteString("/.")
	}
	out.WriteString(u.Pathname())
	if u.hasQuery {
		out.WriteByte('?')
		out.WriteString(u.query)
	}
	if !excludeFragment && u.hasFragment {
		out.WriteByte('#')
		out.WriteString(u.fragment)
	}
	return out.String()
}
func (u *URL) Origin() string {
	switch u.scheme {
	case "http", "https", "ws", "wss", "ftp":
		return u.Protocol() + "//" + u.Host()
	case "blob":
		inner, err := Parse(u.opaque)
		if err == nil && (inner.scheme == "http" || inner.scheme == "https") {
			return inner.Origin()
		}
	}
	return "null"
}
func special(scheme string) bool {
	switch scheme {
	case "http", "https", "ws", "wss", "ftp", "file":
		return true
	}
	return false
}
func defaultPort(scheme string) string {
	switch scheme {
	case "http", "ws":
		return "80"
	case "https", "wss":
		return "443"
	case "ftp":
		return "21"
	}
	return ""
}
func alpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func schemeEnd(value string) int {
	if value == "" || !alpha(value[0]) {
		return -1
	}
	for i := 1; i < len(value); i++ {
		c := value[i]
		if c == ':' {
			return i
		}
		if !alpha(c) && !(c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.') {
			break
		}
	}
	return -1
}
func clean(value string) string {
	value = strings.TrimFunc(value, func(r rune) bool { return r <= 32 })
	if strings.ContainsAny(value, "\t\n\r") {
		value = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(value)
	}
	return scalar(value)
}

// scalar applies Web IDL's USVString boundary to UTF8 or the wire's WTF8.
func scalar(value string) string {
	if utf8.ValidString(value) {
		return value
	}
	var out strings.Builder
	for i := 0; i < len(value); {
		if i+2 < len(value) && value[i] == 0xed && value[i+1] >= 0xa0 && value[i+1] <= 0xbf && value[i+2]&0xc0 == 0x80 {
			unit := rune(value[i]&15)<<12 | rune(value[i+1]&63)<<6 | rune(value[i+2]&63)
			i += 3
			if unit <= 0xdbff && i+2 < len(value) && value[i] == 0xed && value[i+1] >= 0xb0 && value[i+1] <= 0xbf && value[i+2]&0xc0 == 0x80 {
				low := rune(value[i]&15)<<12 | rune(value[i+1]&63)<<6 | rune(value[i+2]&63)
				out.WriteRune(0x10000 + (unit-0xd800)*1024 + low - 0xdc00)
				i += 3
			} else {
				out.WriteRune(utf8.RuneError)
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		out.WriteRune(r)
		i += size
	}
	return out.String()
}
func Parse(value string) (*URL, error) { return parse(clean(value), nil) }
func ParseRef(base, value string) (*URL, error) {
	parsed, err := Parse(base)
	if err != nil {
		return nil, err
	}
	return parsed.Parse(value)
}
func (u *URL) Parse(value string) (*URL, error) { return parse(clean(value), u) }
func copyURL(base *URL) *URL {
	u := *base
	u.path = append([]string(nil), base.path...)
	u.fragment = ""
	u.hasFragment = false
	return &u
}
func slash(c byte, isSpecial bool) bool { return c == '/' || isSpecial && c == '\\' }
func parse(value string, base *URL) (*URL, error) {
	end := schemeEnd(value)
	if end >= 0 {
		scheme := lower(value[:end])
		rest := value[end+1:]
		if scheme == "file" {
			return parseFile(rest, base)
		}
		if special(scheme) {
			if base != nil && base.scheme == scheme && !(len(rest) >= 2 && rest[0] == '/' && rest[1] == '/') {
				return relative(rest, base)
			}
			for len(rest) > 0 && slash(rest[0], true) {
				rest = rest[1:]
			}
			u := &URL{scheme: scheme, hasHost: true}
			return authorityAndTail(u, rest, true)
		}
		u := &URL{scheme: scheme}
		if strings.HasPrefix(rest, "//") {
			u.hasHost = true
			return authorityAndTail(u, rest[2:], false)
		}
		if strings.HasPrefix(rest, "/") {
			return tail(u, rest[1:], true)
		}
		u.opaquePath = true
		body, query, fragment, hq, hf := splitTail(rest)
		// The initial whole-input trim already handles opaque trailing spaces when
		// there is no query/fragment. Interior opaque spaces remain literal.
		u.opaque = percent(body, encodeC0)
		if (hq || hf) && strings.HasSuffix(body, " ") {
			u.opaque = u.opaque[:len(u.opaque)-1] + "%20"
		}
		u.query = percent(query, encodeQuery)
		u.fragment = percent(fragment, encodeFragment)
		u.hasQuery = hq
		u.hasFragment = hf
		return u, nil
	}
	if base == nil {
		return nil, errURL
	}
	if base.opaquePath {
		if !strings.HasPrefix(value, "#") {
			return nil, errURL
		}
		u := copyURL(base)
		u.fragment = percent(value[1:], encodeFragment)
		u.hasFragment = true
		return u, nil
	}
	if base.scheme == "file" {
		return parseFile(value, base)
	}
	return relative(value, base)
}
func relative(value string, base *URL) (*URL, error) {
	u := copyURL(base)
	isSpecial := special(u.scheme)
	if len(value) > 0 && slash(value[0], isSpecial) {
		if len(value) > 1 && slash(value[1], isSpecial) {
			rest := value[2:]
			if isSpecial {
				for len(rest) > 0 && slash(rest[0], true) {
					rest = rest[1:]
				}
			}
			u.username = ""
			u.password = ""
			u.hostname = ""
			u.port = ""
			u.path = nil
			u.hasHost = true
			u.query = ""
			u.hasQuery = false
			return authorityAndTail(u, rest, isSpecial)
		}
		u.path = nil
		u.query = ""
		u.hasQuery = false
		return tail(u, value[1:], true)
	}
	if value == "" {
		return u, nil
	}
	if value[0] == '#' {
		u.fragment = percent(value[1:], encodeFragment)
		u.hasFragment = true
		return u, nil
	}
	if value[0] == '?' {
		return tail(u, value, false)
	}
	shorten(u)
	u.query = ""
	u.hasQuery = false
	return tail(u, value, true)
}
func splitTail(value string) (body, query, fragment string, hq, hf bool) {
	body = value
	if at := strings.IndexByte(body, '#'); at >= 0 {
		fragment = body[at+1:]
		body = body[:at]
		hf = true
	}
	if at := strings.IndexByte(body, '?'); at >= 0 {
		query = body[at+1:]
		body = body[:at]
		hq = true
	}
	return
}
func tail(u *URL, value string, withPath bool) (*URL, error) {
	body, query, fragment, hq, hf := splitTail(value)
	if withPath {
		appendPath(u, body)
	}
	if hq {
		kind := encodeQuery
		if special(u.scheme) {
			kind = encodeSpecialQuery
		}
		u.query = percent(query, kind)
		u.hasQuery = true
	}
	u.fragment = percent(fragment, encodeFragment)
	u.hasFragment = hf
	return u, nil
}
func authorityAndTail(u *URL, value string, isSpecial bool) (*URL, error) {
	end := 0
	for end < len(value) && value[end] != '?' && value[end] != '#' && !slash(value[end], isSpecial) {
		end++
	}
	authority := value[:end]
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		user := authority[:at]
		authority = authority[at+1:]
		if colon := strings.IndexByte(user, ':'); colon >= 0 {
			u.username = percent(user[:colon], encodeUserinfo)
			u.password = percent(user[colon+1:], encodeUserinfo)
		} else {
			u.username = percent(user, encodeUserinfo)
		}
		if authority == "" {
			return nil, errURL
		}
	}
	host, port := authority, ""
	if strings.HasPrefix(host, "[") {
		close := strings.IndexByte(host, ']')
		if close < 0 {
			return nil, errURL
		}
		suffix := host[close+1:]
		host = host[:close+1]
		if suffix != "" {
			if suffix[0] != ':' {
				return nil, errURL
			}
			port = suffix[1:]
		}
	} else if at := strings.LastIndexByte(host, ':'); at >= 0 {
		port = host[at+1:]
		host = host[:at]
	}
	if isSpecial && host == "" {
		return nil, errURL
	}
	if host == "" && (strings.ContainsRune(authority, ':') || u.username != "" || u.password != "") {
		return nil, errURL
	}
	parsed, err := parseHost(host, !isSpecial)
	if err != nil {
		return nil, err
	}
	u.hostname = parsed
	if port != "" {
		for i := 0; i < len(port); i++ {
			if port[i] < '0' || port[i] > '9' {
				return nil, errURL
			}
		}
		p, err := strconv.ParseUint(strings.TrimLeft(port, "0"), 10, 16)
		if strings.TrimLeft(port, "0") == "" {
			p = 0
			err = nil
		}
		if err != nil {
			return nil, errURL
		}
		u.port = strconv.FormatUint(p, 10)
		if u.port == defaultPort(u.scheme) {
			u.port = ""
		}
	}
	rest := value[end:]
	if len(rest) > 0 && slash(rest[0], isSpecial) {
		return tail(u, rest[1:], true)
	}
	if isSpecial {
		u.path = []string{""}
	}
	return tail(u, rest, false)
}
func drive(value string) bool {
	return len(value) == 2 && alpha(value[0]) && (value[1] == ':' || value[1] == '|')
}
func startsDrive(value string) bool {
	return len(value) >= 2 && drive(value[:2]) && (len(value) == 2 || strings.ContainsRune("/\\?#", rune(value[2])))
}
func shorten(u *URL) {
	if len(u.path) > 0 && !(u.scheme == "file" && len(u.path) == 1 && drive(u.path[0])) {
		u.path = u.path[:len(u.path)-1]
	}
}
func appendPath(u *URL, value string) {
	if special(u.scheme) {
		value = strings.ReplaceAll(value, "\\", "/")
	}
	parts := strings.Split(value, "/")
	for i, part := range parts {
		folded := lower(part)
		last := i == len(parts)-1
		if folded == ".." || folded == ".%2e" || folded == "%2e." || folded == "%2e%2e" {
			shorten(u)
			if last {
				u.path = append(u.path, "")
			}
			continue
		}
		if folded == "." || folded == "%2e" {
			if last {
				u.path = append(u.path, "")
			}
			continue
		}
		if u.scheme == "file" && len(u.path) == 0 && drive(part) {
			part = part[:1] + ":"
		}
		u.path = append(u.path, percent(part, encodePath))
	}
}
func parseFile(value string, base *URL) (*URL, error) {
	u := &URL{scheme: "file", hasHost: true}
	if base != nil && base.scheme != "file" {
		base = nil
	}
	if len(value) >= 2 && slash(value[0], true) && slash(value[1], true) {
		rest := value[2:]
		end := 0
		for end < len(rest) && !slash(rest[end], true) && rest[end] != '?' && rest[end] != '#' {
			end++
		}
		host := rest[:end]
		if drive(host) {
			return tail(u, rest, true)
		}
		parsed, err := parseHost(host, false)
		if err != nil {
			return nil, err
		}
		if parsed == "localhost" {
			parsed = ""
		}
		u.hostname = parsed
		rest = rest[end:]
		if len(rest) > 0 && slash(rest[0], true) {
			rest = rest[1:]
		}
		return tail(u, rest, true)
	}
	if base != nil {
		u = copyURL(base)
		if value == "" {
			return u, nil
		}
		if value[0] == '#' {
			u.fragment = percent(value[1:], encodeFragment)
			u.hasFragment = true
			return u, nil
		}
		if value[0] == '?' {
			return tail(u, value, false)
		}
		u.query = ""
		u.hasQuery = false
		if slash(value[0], true) {
			u.path = nil
			if !startsDrive(value[1:]) && len(base.path) > 0 && drive(base.path[0]) {
				u.path = append(u.path, base.path[0])
			}
			return tail(u, value[1:], true)
		}
		if startsDrive(value) {
			u.path = nil
		} else {
			shorten(u)
		}
		return tail(u, value, true)
	}
	if len(value) > 0 && slash(value[0], true) {
		value = value[1:]
	}
	return tail(u, value, true)
}
