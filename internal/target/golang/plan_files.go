package golang

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"openapi-sdkgen/internal/generator"
)

type runtimeModule struct {
	Path, Package, Template string
	Sources                 []string
	Dependencies            []string
	StandardImports         []importPlan
}

// This catalog is explicit: adding an embedded file never changes output roots.
var runtimeModules = map[string]runtimeModule{
	"owned-unicode-license": {Path: "internal/runtime/unicode17/LICENSE.txt", Package: "unicode17", Sources: []string{"unicode17/LICENSE.txt"}},
	"owned-fold":            {Path: "internal/runtime/unicode17/fold.go", Package: "unicode17", Sources: []string{"unicode17/ranges.go", "unicode17/fold.go", "unicode17/fold_data.go"}, Dependencies: []string{"owned-unicode-license"}},
	"owned-properties":      {Path: "internal/runtime/unicode17/properties.go", Package: "unicode17", Sources: []string{"unicode17/properties.go", "unicode17/properties_data.go"}, Dependencies: []string{"owned-fold"}},
	"owned-idna":            {Path: "internal/runtime/unicode17/idna.go", Package: "unicode17", Sources: []string{"unicode17/idna.go", "unicode17/idna_data.go", "unicode17/normalization.go", "unicode17/normalization_data.go"}, Dependencies: []string{"owned-unicode-license"}},
	"owned-url":             {Path: "internal/runtime/urlstd/url.go", Package: "urlstd", Sources: []string{"urlstd/url.go", "urlstd/host.go", "urlstd/idna.go", "urlstd/punycode.go", "urlstd/encoding.go"}, Dependencies: []string{"owned-idna"}},
	"owned-regex":           {Path: "internal/runtime/ecmaregexp/program.go", Package: "ecmaregexp", Sources: []string{"ecmaregexp/program.go", "ecmaregexp/ranges.go", "ecmaregexp/text.go", "ecmaregexp/match.go", "ecmaregexp/scratch.go"}, Dependencies: []string{"owned-fold"}},
	"owned-compiler":        {Path: "internal/runtime/ecmaregexp/compile.go", Package: "ecmaregexp", Sources: []string{"ecmaregexp/compile.go", "ecmaregexp/classes.go"}, Dependencies: []string{"owned-regex", "owned-properties"}},
	"regex-compiler":        {Path: "internal/schema/regex_compile.go", Package: "schema", Template: "runtime/regex-compiler.go.tmpl", Dependencies: []string{"regex", "owned-compiler", "wire"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}}},

	"server-protocols":              {Path: "server/runtime/protocols.go", Package: "runtime", Template: "runtime/server-protocols.go.tmpl", Dependencies: []string{"stream-json", "stream-sse", "stream-multipart"}},
	"server-schema-contract":        {Path: "server/runtime/schema_contract.go", Package: "runtime", Template: "runtime/server-schema-contract.go.tmpl", Dependencies: []string{"schema-integer", "types"}, StandardImports: []importPlan{{Path: "fmt"}}},
	"server-schema-xml":             {Path: "server/runtime/schema_xml.go", Package: "runtime", Template: "runtime/server-schema-xml.go.tmpl", Dependencies: []string{"server-media", "schema", "wire-xml-output", "types"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}}},
	"server-multipart-output":       {Path: "server/runtime/multipart_output.go", Package: "runtime", Template: "runtime/server-multipart-output.go.tmpl", Dependencies: []string{"server-multipart", "server-stream", "multipart-encode", "media-multipart-source", "stream-multipart", "types-multipart"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "io"}, {Path: "strings"}}},
	"multipart-encode":              {Path: "internal/multipartcodec/encode.go", Package: "multipartcodec", Template: "runtime/multipart-encode.go.tmpl", Dependencies: []string{"multipart-codec", "media-multipart", "wire", "types-multipart"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "fmt"}, {Path: "io"}, {Path: "net/http"}, {Path: "mime"}, {Path: "net/textproto"}, {Path: "strconv"}, {Path: "strings"}}},
	"server-multipart":              {Path: "server/runtime/multipart.go", Package: "runtime", Template: "runtime/server-multipart.go.tmpl", Dependencies: []string{"server-media", "multipart-codec", "types-multipart", "schema", "stream-multipart", "types-stream-data"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "fmt"}, {Path: "io"}, {Path: "mime"}, {Path: "mime/multipart"}, {Path: "net/http"}, {Path: "strconv"}, {Path: "strings"}}},
	"multipart-codec":               {Path: "internal/multipartcodec/decode.go", Package: "multipartcodec", Template: "runtime/multipart-codec.go.tmpl", Dependencies: []string{"stream-multipart", "media-inbound", "types-multipart", "types-stream-data"}, StandardImports: []importPlan{{Path: "bytes"}, {Path: "context"}, {Path: "errors"}, {Path: "fmt"}, {Path: "io"}, {Path: "mime"}, {Path: "net/http"}, {Path: "sort"}, {Path: "strconv"}, {Path: "strings"}}},
	"server-schema":                 {Path: "server/runtime/schema.go", Package: "runtime", Template: "runtime/server-schema.go.tmpl", Dependencies: []string{"server-schema-xml", "server-schema-contract", "server-media", "media-value", "schema-integer", "schema-multiple", "schema-unique", "schema-content", "regex-compiler", "wire-xml", "types"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "sort"}}},
	"server-headers":                {Path: "server/runtime/headers.go", Package: "runtime", Template: "runtime/server-headers.go.tmpl", Dependencies: []string{"server-media", "server-parameters", "media-value", "types"}, StandardImports: []importPlan{{Path: "fmt"}, {Path: "net/http"}, {Path: "net/url"}, {Path: "strings"}}},
	"server-stream":                 {Path: "server/runtime/stream.go", Package: "runtime", Template: "runtime/server-stream.go.tmpl", Dependencies: []string{"server-execution", "stream-reader", "types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "fmt"}, {Path: "io"}, {Path: "sync"}}},
	"server-media":                  {Path: "server/runtime/media.go", Package: "runtime", Template: "runtime/server-media.go.tmpl", Dependencies: []string{"server-execution", "server-coercion", "media-inbound", "media-form", "wire", "types"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "io"}, {Path: "strconv"}, {Path: "strings"}}},
	"server-parameters":             {Path: "server/runtime/parameters.go", Package: "runtime", Template: "runtime/server-parameters.go.tmpl", Dependencies: []string{"server-execution", "server-coercion", "types"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "net/url"}, {Path: "strings"}}},
	"server-coercion":               {Path: "server/runtime/coercion.go", Package: "runtime", Template: "runtime/server-coercion.go.tmpl", Dependencies: []string{"server-types", "schema", "wire-scalar", "types"}, StandardImports: []importPlan{{Path: "context"}, {Path: "strings"}}},
	"server-types":                  {Path: "server/runtime/types.go", Package: "runtime", Template: "runtime/server-types.go.tmpl", Dependencies: []string{"media-form", "types-request"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "io"}, {Path: "math"}, {Path: "mime"}, {Path: "net/http"}, {Path: "strings"}, {Path: "sync"}}},
	"server-execution":              {Path: "server/runtime/execution.go", Package: "runtime", Template: "runtime/server-execution.go.tmpl", Dependencies: []string{"server-types", "server-security", "media-http", "types-request"}, StandardImports: []importPlan{{Path: "bytes"}, {Path: "context"}, {Path: "errors"}, {Path: "fmt"}, {Path: "io"}, {Path: "net/http"}, {Path: "sync"}}},
	"server-security":               {Path: "server/runtime/security.go", Package: "runtime", Template: "runtime/server-security.go.tmpl", Dependencies: []string{"server-types", "wire"}, StandardImports: []importPlan{{Path: "net/http"}, {Path: "net/url"}, {Path: "sort"}, {Path: "strings"}, {Path: "unicode/utf8"}}},
	"server-handler":                {Path: "server/runtime/handler.go", Package: "runtime", Template: "runtime/server-handler.go.tmpl", Dependencies: []string{"server-execution", "server-security", "types-binary"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "fmt"}, {Path: "io"}, {Path: "mime"}, {Path: "net/http"}, {Path: "net/url"}, {Path: "regexp"}, {Path: "strconv"}, {Path: "strings"}}},
	"core-link-sources":             {Path: "internal/core/link_sources.go", Package: "core", Template: "runtime/core-link-sources.go.tmpl", Dependencies: []string{"core-links", "types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "strconv"}, {Path: "sync/atomic"}}},
	"core-links":                    {Path: "internal/core/links.go", Package: "core", Template: "runtime/core-links.go.tmpl", Dependencies: []string{"core-response", "types-links", "types-native-input"}, StandardImports: []importPlan{{Path: "context"}, {Path: "net/http"}, {Path: "net/url"}, {Path: "strconv"}, {Path: "strings"}}},
	"types-native-input":            {Path: "types/native_input.go", Package: "types", Template: "runtime/types-native-input.go.tmpl", Dependencies: []string{"types-multipart", "types-binary", "wire-media"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}}},
	"types-links":                   {Path: "types/links.go", Package: "types", Template: "runtime/types-links.go.tmpl", Dependencies: []string{"types"}},
	"types-pagination":              {Path: "types/pagination.go", Package: "types", Template: "runtime/types-pagination.go.tmpl", Dependencies: []string{"types-stream"}, StandardImports: []importPlan{{Path: "context"}, {Path: "net/http"}, {Path: "strconv"}, {Path: "sync"}}},
	"types-stream-data":             {Path: "types/stream_data.go", Package: "types", Template: "runtime/types-stream-data.go.tmpl", Dependencies: []string{"types-stream-codec", "types-multipart", "wire"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "io"}}},
	"media-multipart-source":        {Path: "internal/media/multipart_source.go", Package: "media", Template: "runtime/media-multipart-source.go.tmpl", Dependencies: []string{"media-multipart", "types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "io"}, {Path: "mime"}}},
	"core-stream-multipart-native":  {Path: "internal/core/stream_multipart_native.go", Package: "core", Template: "runtime/core-stream-multipart-native.go.tmpl", Dependencies: []string{"core-stream-multipart-request", "media-multipart-source", "types-multipart", "multipart-encode"}, StandardImports: []importPlan{{Path: "context"}, {Path: "io"}}},
	"core-stream-request":           {Path: "internal/core/stream_request.go", Package: "core", Template: "runtime/core-stream-request.go.tmpl", Dependencies: []string{"core-stream", "types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "io"}, {Path: "sync"}, {Path: "sync/atomic"}}},
	"core-stream-multipart-request": {Path: "internal/core/stream_multipart_request.go", Package: "core", Template: "runtime/core-stream-multipart-request.go.tmpl", Dependencies: []string{"core-multipart", "core-stream-request", "types-multipart", "types-stream-contract", "media-multipart"}, StandardImports: []importPlan{{Path: "context"}, {Path: "strconv"}, {Path: "strings"}, {Path: "sync/atomic"}}},
	"core-stream":                   {Path: "internal/core/stream.go", Package: "core", Template: "runtime/core-stream.go.tmpl", Dependencies: []string{"types-stream-data", "core-execution", "core-options", "core-response"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "net/http"}, {Path: "strconv"}}},
	"core-stream-part":              {Path: "internal/core/stream_part.go", Package: "core", Template: "runtime/core-stream-part.go.tmpl", Dependencies: []string{"core-stream", "core-body", "multipart-codec", "types-stream-contract"}, StandardImports: []importPlan{{Path: "context"}}},
	"stream-multipart":              {Path: "internal/stream/multipart.go", Package: "stream", Template: "runtime/stream-multipart.go.tmpl", Dependencies: []string{"stream-reader", "stream-encode", "media-multipart", "types-stream-contract", "wire"}, StandardImports: []importPlan{{Path: "bytes"}, {Path: "context"}, {Path: "errors"}, {Path: "io"}, {Path: "mime"}, {Path: "net/http"}, {Path: "net/textproto"}, {Path: "strings"}, {Path: "sync/atomic"}}},
	"stream-sse":                    {Path: "internal/stream/sse.go", Package: "stream", Template: "runtime/stream-sse.go.tmpl", Dependencies: []string{"stream-reader", "stream-encode", "wire-text-stream", "types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "io"}, {Path: "math"}, {Path: "strconv"}, {Path: "strings"}, {Path: "sync/atomic"}}},
	"wire-text-stream":              {Path: "internal/wire/text_stream.go", Package: "wire", Template: "runtime/wire-text-stream.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "unicode/utf8"}}},
	"stream-json":                   {Path: "internal/stream/json.go", Package: "stream", Template: "runtime/stream-json.go.tmpl", Dependencies: []string{"stream-reader", "stream-encode", "wire-text-stream", "types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "io"}, {Path: "strings"}, {Path: "sync/atomic"}}},
	"stream-encode":                 {Path: "internal/stream/encode.go", Package: "stream", Template: "runtime/stream-encode.go.tmpl", Dependencies: []string{"types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "io"}, {Path: "sync"}, {Path: "sync/atomic"}}},
	"types-stream":                  {Path: "types/stream.go", Package: "types", Template: "runtime/types-stream.go.tmpl", Dependencies: []string{"types-stream-codec", "types-request"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "net/http"}, {Path: "sync"}, {Path: "sync/atomic"}}},
	"types-stream-contract":         {Path: "types/stream_contract.go", Package: "types", Template: "runtime/types-stream-contract.go.tmpl", Dependencies: []string{"types-binary"}, StandardImports: []importPlan{{Path: "context"}, {Path: "io"}, {Path: "net/http"}}},
	"types-stream-codec":            {Path: "types/stream_codec.go", Package: "types", Template: "runtime/types-stream-codec.go.tmpl", Dependencies: []string{"types-stream-contract"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "io"}, {Path: "sync"}}},
	"stream-reader":                 {Path: "internal/stream/reader.go", Package: "stream", Template: "runtime/stream-reader.go.tmpl", Dependencies: []string{"types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "io"}, {Path: "sync"}, {Path: "sync/atomic"}}},
	"media-http":                    {Path: "internal/media/http.go", Package: "media", Template: "runtime/media-http.go.tmpl", StandardImports: []importPlan{{Path: "context"}, {Path: "io"}, {Path: "net/http"}}},
	"media-inbound":                 {Path: "internal/media/inbound.go", Package: "media", Template: "runtime/media-inbound.go.tmpl", Dependencies: []string{"media-http", "types-request"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "net/http"}}},
	"types-multipart":               {Path: "types/multipart.go", Package: "types", Template: "runtime/types-multipart.go.tmpl", Dependencies: []string{"types-binary", "wire-media"}, StandardImports: []importPlan{{Path: "fmt"}, {Path: "strconv"}, {Path: "strings"}}},
	"media-multipart":               {Path: "internal/media/multipart.go", Package: "media", Template: "runtime/media-multipart.go.tmpl", Dependencies: []string{"types-binary"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "fmt"}, {Path: "io"}, {Path: "mime"}, {Path: "mime/multipart"}, {Path: "net/textproto"}, {Path: "sort"}, {Path: "strings"}, {Path: "sync"}}},
	"core-multipart":                {Path: "internal/core/multipart.go", Package: "core", Template: "runtime/core-multipart.go.tmpl", Dependencies: []string{"core-body", "multipart-encode", "media-multipart", "wire", "types-multipart"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}}},
	"wire-media":                    {Path: "internal/wire/media.go", Package: "wire", Template: "runtime/wire-media.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "strconv"}, {Path: "strings"}}},
	"wire-xml-name":                 {Path: "internal/wire/xml_name.go", Package: "wire", Template: "runtime/wire-xml-name.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "strconv"}, {Path: "strings"}}},
	"wire-xml-output":               {Path: "internal/wire/xml_output.go", Package: "wire", Template: "runtime/wire-xml-output.go.tmpl", Dependencies: []string{"wire-xml"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "strings"}}},
	"media-form":                    {Path: "internal/media/form.go", Package: "media", Template: "runtime/media-form.go.tmpl", Dependencies: []string{"media-value", "types-request"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "strings"}}},
	"transport-url":                 {Path: "internal/transport/url.go", Package: "transport", Template: "runtime/transport-url.go.tmpl", Dependencies: []string{"types-errors", "owned-url"}, StandardImports: []importPlan{{Path: "net/url"}}},
	"types-security":                {Path: "types/security.go", Package: "types", Template: "runtime/types-security.go.tmpl", Dependencies: []string{"types"}, StandardImports: []importPlan{{Path: "context"}}},
	"security":                      {Path: "internal/security/security.go", Package: "security", Template: "runtime/security.go.tmpl", Dependencies: []string{"types-security", "types-request"}, StandardImports: []importPlan{{Path: "context"}, {Path: "encoding/base64"}, {Path: "net/http"}, {Path: "net/url"}, {Path: "strings"}}},
	"core-server":                   {Path: "internal/core/server.go", Package: "core", Template: "runtime/core-server.go.tmpl", Dependencies: []string{"types-security", "transport-url"}, StandardImports: []importPlan{{Path: "net/url"}, {Path: "strings"}}},
	"transport-headers":             {Path: "internal/transport/headers.go", Package: "transport", Template: "runtime/transport-headers.go.tmpl", Dependencies: []string{"types-binary"}, StandardImports: []importPlan{{Path: "io"}, {Path: "net/http"}, {Path: "strconv"}, {Path: "strings"}}},
	"types-media":                   {Path: "types/media.go", Package: "types", Template: "runtime/types-media.go.tmpl", Dependencies: []string{"types-request", "types-multipart"}, StandardImports: []importPlan{{Path: "context"}}},
	"media-value":                   {Path: "internal/media/value.go", Package: "media", Template: "runtime/media-value.go.tmpl", Dependencies: []string{"types-request", "wire"}, StandardImports: []importPlan{{Path: "strings"}}},
	"wire-scalar":                   {Path: "internal/wire/scalar.go", Package: "wire", Template: "runtime/wire-scalar.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "math/big"}, {Path: "strconv"}, {Path: "strings"}, {Path: "unicode"}}},
	"schema-header":                 {Path: "internal/schema/header.go", Package: "schema", Template: "runtime/schema-header.go.tmpl", Dependencies: []string{"wire-scalar"}, StandardImports: []importPlan{{Path: "context"}, {Path: "strings"}}},
	"core-body":                     {Path: "internal/core/body.go", Package: "core", Template: "runtime/core-body.go.tmpl", Dependencies: []string{"types-request", "media-form"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "net/http"}, {Path: "strings"}}},
	"core-response":                 {Path: "internal/core/response.go", Package: "core", Template: "runtime/core-response.go.tmpl", Dependencies: []string{"media-value", "types-request"}, StandardImports: []importPlan{{Path: "context"}, {Path: "sort"}, {Path: "io"}, {Path: "net/http"}, {Path: "strconv"}, {Path: "strings"}}},
	"core-parameters":               {Path: "internal/core/parameters.go", Package: "core", Template: "runtime/core-parameters.go.tmpl", Dependencies: []string{"types-request", "media-value"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "net/http"}, {Path: "strings"}, {Path: "unicode/utf8"}}},
	"core-execution":                {Path: "internal/core/execution.go", Package: "core", Template: "runtime/core-execution.go.tmpl", Dependencies: []string{"transport-http", "types-request", "security", "core-server"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "io"}, {Path: "net/http"}, {Path: "net/url"}, {Path: "strings"}, {Path: "sync"}, {Path: "sync/atomic"}}},
	"types-request":                 {Path: "types/request.go", Package: "types", Template: "runtime/types-request.go.tmpl", Dependencies: []string{"types-binary", "types-transport", "types-security", "types-stream-codec"}, StandardImports: []importPlan{{Path: "context"}, {Path: "net/http"}, {Path: "time"}}},
	"core-options":                  {Path: "internal/core/options.go", Package: "core", Template: "runtime/core-options.go.tmpl", Dependencies: []string{"types-request", "transport-url", "media-http"}, StandardImports: []importPlan{{Path: "sort"}, {Path: "context"}, {Path: "errors"}, {Path: "net"}, {Path: "net/http"}, {Path: "net/url"}, {Path: "strings"}}},
	"transport-http":                {Path: "internal/transport/adapter.go", Package: "transport", Template: "runtime/transport-http.go.tmpl", Dependencies: []string{"types-transport", "transport-body", "transport-headers", "transport-url"}, StandardImports: []importPlan{{Path: "errors"}, {Path: "io"}, {Path: "net/http"}, {Path: "net/url"}, {Path: "strings"}, {Path: "sync"}}},
	"types-transport":               {Path: "types/transport.go", Package: "types", Template: "runtime/types-transport.go.tmpl", Dependencies: []string{"types-errors"}, StandardImports: []importPlan{{Path: "net/http"}}},
	"transport-body":                {Path: "internal/transport/body.go", Package: "transport", Template: "runtime/transport-body.go.tmpl", Dependencies: []string{"types-transport"}, StandardImports: []importPlan{{Path: "context"}, {Path: "io"}, {Path: "net/http"}, {Path: "sort"}, {Path: "strings"}, {Path: "sync"}, {Path: "sync/atomic"}}},
	"types-errors":                  {Path: "types/errors.go", Package: "types", Template: "runtime/types-errors.go.tmpl", Dependencies: []string{"types"}, StandardImports: []importPlan{{Path: "net/http"}, {Path: "errors"}, {Path: "strings"}, {Path: "sort"}}},
	"types-binary":                  {Path: "types/binary.go", Package: "types", Template: "runtime/types-binary.go.tmpl", Dependencies: []string{"types", "types-errors", "wire"}, StandardImports: []importPlan{{Path: "bytes"}, {Path: "context"}, {Path: "errors"}, {Path: "io"}, {Path: "reflect"}, {Path: "strings"}, {Path: "sync"}, {Path: "sync/atomic"}}},
	"wire-literal":                  {Path: "internal/wire/literal.go", Package: "wire", Template: "runtime/wire-literal.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "sync"}}},
	"wire-xml":                      {Path: "internal/wire/xml.go", Package: "wire", Template: "runtime/wire-xml.go.tmpl", Dependencies: []string{"wire-scalar", "wire-xml-name"}, StandardImports: []importPlan{{Path: "context"}, {Path: "encoding/xml"}, {Path: "fmt"}, {Path: "io"}, {Path: "strings"}}},
	"schema-content":                {Path: "internal/schema/content.go", Package: "schema", Template: "runtime/schema-content.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "encoding/base64"}, {Path: "strings"}}},
	"schema-integer":                {Path: "internal/schema/integer.go", Package: "schema", Template: "runtime/schema-integer.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "math"}}},
	"schema-multiple":               {Path: "internal/schema/multiple.go", Package: "schema", Template: "runtime/schema-multiple.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "math"}, {Path: "math/big"}, {Path: "strconv"}}},
	"schema-unique":                 {Path: "internal/schema/unique.go", Package: "schema", Template: "runtime/schema-unique.go.tmpl", Dependencies: []string{"wire-hash"}, StandardImports: []importPlan{{Path: "context"}, {Path: "hash/maphash"}}},
	"wire-hash":                     {Path: "internal/wire/hash.go", Package: "wire", Template: "runtime/wire-hash.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "context"}, {Path: "encoding/binary"}, {Path: "hash/maphash"}, {Path: "math"}}},
	"format-number":                 {Path: "internal/schema/format-number.go", Package: "schema", Template: "runtime/format-number.go.tmpl"},
	"format-date":                   {Path: "internal/schema/format-date.go", Package: "schema", Template: "runtime/format-date.go.tmpl", Dependencies: []string{"format-number"}},
	"format-time":                   {Path: "internal/schema/format-time.go", Package: "schema", Template: "runtime/format-time.go.tmpl", Dependencies: []string{"format-number"}},
	"format-date-time":              {Path: "internal/schema/format-date-time.go", Package: "schema", Template: "runtime/format-date-time.go.tmpl", Dependencies: []string{"format-date", "format-time"}},
	"format-ip":                     {Path: "internal/schema/format-ip.go", Package: "schema", Template: "runtime/format-ip.go.tmpl", StandardImports: []importPlan{{Path: "net/netip"}, {Path: "strings"}}},
	"format-template":               {Path: "internal/schema/format-template.go", Package: "schema", Template: "runtime/format-template.go.tmpl"},
	"format-url":                    {Path: "internal/schema/format-url.go", Package: "schema", Template: "runtime/format-url.go.tmpl", Dependencies: []string{"owned-url"}, StandardImports: []importPlan{{Path: "strings"}, {Path: "sync"}}},
	"model-state":                   {Path: "models/state.go", Package: "models", Template: "runtime/model-state.go.tmpl", Dependencies: []string{"schema", "types-binary", "wire-media"}, StandardImports: []importPlan{{Path: "context"}, {Path: "fmt"}, {Path: "strings"}}},
	"regex":                         {Path: "internal/schema/regex.go", Package: "schema", Template: "runtime/regex.go.tmpl", Dependencies: []string{"wire", "owned-regex"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}}},
	"wire":                          {Path: "internal/wire/json.go", Package: "wire", Template: "runtime/wire.go.tmpl", Dependencies: []string{"wire-value"}, StandardImports: []importPlan{{Path: "fmt"}, {Path: "strconv"}, {Path: "strings"}, {Path: "unicode/utf8"}}},
	"wire-value":                    {Path: "internal/wire/value.go", Package: "wire", Template: "runtime/wire-value.go.tmpl", StandardImports: []importPlan{{Path: "math"}, {Path: "fmt"}, {Path: "sort"}, {Path: "strconv"}, {Path: "strings"}, {Path: "unicode/utf8"}}},
	"types":                         {Path: "types/value.go", Package: "types", Template: "runtime/types.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "fmt"}, {Path: "reflect"}}},
	"schema":                        {Path: "internal/schema/json.go", Package: "schema", Template: "runtime/schema.go.tmpl", Dependencies: []string{"wire"}, StandardImports: []importPlan{{Path: "context"}, {Path: "errors"}, {Path: "fmt"}, {Path: "strconv"}, {Path: "strings"}}},
	"core":                          {Path: "internal/core/client.go", Package: "core", Template: "runtime/core.go.tmpl", Dependencies: []string{"transport-http", "types-request", "core-options", "core-execution"}, StandardImports: []importPlan{{Path: "fmt"}, {Path: "net/url"}, {Path: "reflect"}, {Path: "strings"}}},
}

func planFiles(config generator.GoOptions, operations []operationPlan, needsRegex, needsModels bool, extraRoots ...string) ([]filePlan, error) {
	files := []filePlan{
		{Path: "client.go", Template: "templates/client.go.tmpl", Package: config.Package, Imports: []importPlan{{Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/internal/operations"}, {Path: config.ImportPath + "/types"}}, Operations: operations},
		{Path: "internal/operations/operations.go", Template: "templates/operation-group.go.tmpl", Package: "operations", Imports: []importPlan{{Path: config.ImportPath + "/internal/core"}}},
	}
	clientModels := false
	for _, operation := range operations {
		clientModels = clientModels || operation.HasInput || operation.HasHeaders || operation.HasTypedStream && !operation.StreamUnion || !operation.OutputUnion && strings.Contains(operation.OutputType, "models.")
		files = append(files, operationFiles(config, operation)...)
	}
	if clientModels {
		files[0].Imports = append(files[0].Imports, importPlan{Path: config.ImportPath + "/models"})
	}
	state := make(map[string]uint8)
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("Go runtime dependency cycle at %q", name)
		}
		if state[name] == 2 {
			return nil
		}
		module, ok := runtimeModules[name]
		if !ok {
			return fmt.Errorf("missing Go runtime module %q", name)
		}
		state[name] = 1
		imports := append([]importPlan(nil), module.StandardImports...)
		for _, dependency := range module.Dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
			if len(module.Sources) == 0 && path.Dir(module.Path) != path.Dir(runtimeModules[dependency].Path) {
				imports = append(imports, importPlan{Path: config.ImportPath + "/" + path.Dir(runtimeModules[dependency].Path)})
			}
		}
		if len(module.Sources) > 0 {
			owned, err := ownedRuntimePlans(config, module)
			if err != nil {
				return err
			}
			files = append(files, owned...)
		} else {
			files = append(files, filePlan{Path: module.Path, Package: module.Package, Template: module.Template, Imports: imports})
		}
		state[name] = 2
		return nil
	}
	roots := []string{"types", "types-binary", "types-transport", "schema", "core", "core-parameters", "core-response", "core-body", "transport-body"}
	for _, operation := range operations {
		if operation.Pagination != nil {
			roots = append(roots, "types-pagination")
		}
		for _, request := range operation.RequestStreams {
			roots = append(roots, "core-stream-request")
			if request.ItemValidation != "" {
				roots = append(roots, "types-stream")
			}
			switch request.Framing {
			case "line-delimited-json", "json-sequence":
				roots = append(roots, "stream-json")
			case "sse":
				roots = append(roots, "stream-sse")
			case "multipart":
				roots = append(roots, "stream-multipart", "core-stream-multipart-native")
			}
		}
		if operation.HasTypedStream {
			roots = append(roots, "types-stream")
		}
		for _, response := range operation.Responses {
			if response.ItemValidation != "" {
				roots = append(roots, "core-stream", "stream-reader")
				switch response.Framing {
				case "line-delimited-json", "json-sequence":
					roots = append(roots, "stream-json")
				case "sse":
					roots = append(roots, "stream-sse")
				case "multipart":
					roots = append(roots, "stream-multipart", "core-stream-part")
				}
			}
		}
		if operation.HasMultipart {
			roots = append(roots, "core-multipart", "types-multipart")
		}
		if operation.HasOpenMedia {
			roots = append(roots, "types-media")
		}
	}
	if needsRegex {
		roots = append(roots, "regex")
	}
	if needsModels {
		roots = append(roots, "model-state")
	}
	roots = append(roots, extraRoots...)
	for _, root := range roots {
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	seen := make(map[string]bool)
	for index := range files {
		file := &files[index]
		if path.Clean(file.Path) != file.Path || strings.HasPrefix(file.Path, "/") || strings.HasPrefix(file.Path, "../") || seen[strings.ToLower(file.Path)] || !validPackage(file.Package) {
			return nil, fmt.Errorf("invalid or colliding Go output %q", file.Path)
		}
		seen[strings.ToLower(file.Path)] = true
		sort.Slice(file.Imports, func(i, j int) bool { return file.Imports[i].Path < file.Imports[j].Path })
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if err := validateFileImports(config.ImportPath, files); err != nil {
		return nil, err
	}
	return files, nil
}

func validateFileImports(root string, files []filePlan) error {
	packages := make(map[string]string)
	dependencies := make(map[string][]string)
	for _, file := range files {
		directory := path.Dir(file.Path)
		importPath := root
		if directory != "." {
			importPath += "/" + directory
		}
		if previous, ok := packages[importPath]; ok && previous != file.Package {
			return fmt.Errorf("conflicting Go packages in %s", directory)
		}
		packages[importPath] = file.Package
		for _, imported := range file.Imports {
			if imported.Path == root || strings.HasPrefix(imported.Path, root+"/") {
				dependencies[importPath] = append(dependencies[importPath], imported.Path)
			}
		}
	}
	state := make(map[string]uint8)
	var visit func(string) error
	visit = func(importPath string) error {
		if _, ok := packages[importPath]; !ok {
			return fmt.Errorf("missing generated Go import %q", importPath)
		}
		if state[importPath] == 1 {
			return fmt.Errorf("generated Go import cycle at %q", importPath)
		}
		if state[importPath] == 2 {
			return nil
		}
		state[importPath] = 1
		for _, dependency := range dependencies[importPath] {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[importPath] = 2
		return nil
	}
	for _, file := range files {
		importPath := root
		if directory := path.Dir(file.Path); directory != "." {
			importPath += "/" + directory
		}
		if err := visit(importPath); err != nil {
			return err
		}
	}
	return nil
}

func completeFiles(config generator.GoOptions, files []filePlan, programs []schemaProgram, models []modelPlan) ([]filePlan, error) {
	programGroups := make(map[string][]schemaProgram)
	for _, program := range programs {
		programGroups[program.Owner] = append(programGroups[program.Owner], program)
	}
	patterns := make(map[string]schemaPattern)
	for _, program := range programs {
		for _, pattern := range program.Patterns {
			if previous, ok := patterns[pattern.Name]; ok && previous.Pattern != pattern.Pattern {
				return nil, fmt.Errorf("conflicting prepared pattern binding %s", pattern.Name)
			}
			patterns[pattern.Name] = pattern
		}
	}
	for _, name := range sortedKeys(patterns) {
		// A separate data artifact keeps formatter memory local to one program;
		// global initialization is literal-only so the linker can discard it.
		files = append(files, filePlan{Path: namedSourceFilePath("internal/schema", "pattern", name), Package: "schema", Template: "templates/prepared.go.tmpl", Imports: []importPlan{{Path: config.ImportPath + "/internal/runtime/ecmaregexp"}}, Prepared: fmt.Sprintf("var %s = Pattern{compiled:%s}\n", name, patterns[name].Program)})
	}

	for _, owner := range sortedKeys(programGroups) {
		group := programGroups[owner]
		imports := []importPlan{{Path: "context"}, {Path: config.ImportPath + "/internal/wire"}}

		for _, program := range group {
			if program.ViewTarget == "" {
				imports = append(imports, importPlan{Path: "errors"})
				break
			}
		}
		sort.Slice(imports, func(i, j int) bool { return imports[i].Path < imports[j].Path })
		files = append(files, filePlan{Path: sourceFilePath("internal/schema", owner), Package: "schema", Template: "templates/schema.go.tmpl", Imports: imports, Programs: group})
	}
	modelGroups := make(map[string][]modelPlan)
	for _, model := range models {
		modelGroups[model.Owner] = append(modelGroups[model.Owner], model)
	}
	for _, owner := range sortedKeys(modelGroups) {
		group := modelGroups[owner]
		imports := []importPlan{{Path: "context"}, {Path: config.ImportPath + "/internal/schema"}, {Path: config.ImportPath + "/types"}}
		if group[0].UsesFmt {
			imports = append(imports, importPlan{Path: "fmt"})
		}
		if group[0].UsesWire {
			imports = append(imports, importPlan{Path: config.ImportPath + "/internal/wire"})
		}
		sort.Slice(imports, func(i, j int) bool { return imports[i].Path < imports[j].Path })
		files = append(files, filePlan{Path: sourceFilePath("models", owner), Package: "models", Template: "templates/model-codecs.go.tmpl", Imports: imports, Models: group})
	}
	seen := make(map[string]bool)
	for _, file := range files {
		key := strings.ToLower(file.Path)
		if path.Clean(file.Path) != file.Path || strings.HasPrefix(file.Path, "/") || strings.HasPrefix(file.Path, "../") || seen[key] || !validPackage(file.Package) {
			return nil, fmt.Errorf("invalid or colliding Go output %q", file.Path)
		}
		seen[key] = true
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if err := validateFileImports(config.ImportPath, files); err != nil {
		return nil, err
	}
	return files, nil
}
