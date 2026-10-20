// Package compatinput prepares explicitly declared provider authoring sources.
// It is not part of the generator's accepted OpenAPI input contract.
package compatinput

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v4"
	"net/url"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/openapiwalk"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const OperationReferencesV1 = "operation-references-v1"

type Spec struct {
	Kind       string `json:"kind"`
	SHA256     string `json:"sha256"`
	Operations int    `json:"operations,omitempty"`
}
type Evidence struct {
	Kind              string `json:"kind"`
	SourceSHA256      string `json:"sourceSha256"`
	SHA256            string `json:"sha256"`
	Operations        int    `json:"operations"`
	ContractsVerified bool   `json:"contractsVerified"`
}
type Prepared struct {
	Path      string
	Evidence  *Evidence
	contracts []operationContract
	created   bool
}
type parameterContract struct {
	Name, Location string
	Required       bool
}
type responseContract struct {
	Status, Media, Type string
	Schema              bool
}
type operationContract struct {
	Route, ID  string
	Parameters []parameterContract
	Responses  []responseContract
	Body       bool
}
type bundler struct {
	root, entry string
	active      map[string]bool
	document    map[string]any
}

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func Prepare(input string, spec *Spec) (*Prepared, error) {
	result := &Prepared{Path: input}
	if spec == nil {
		return result, nil
	}
	if !SupportedKind(spec.Kind) || len(spec.SHA256) != 64 {
		return nil, fmt.Errorf("unsupported or unpinned input preparation")
	}
	source, err := os.ReadFile(input)
	if err != nil {
		return nil, err
	}
	data, contracts, err := BundleWithKind(input, spec.Kind)
	if err != nil {
		return nil, err
	}
	if spec.Operations > 0 && len(contracts) != spec.Operations {
		return nil, fmt.Errorf("prepared operation count differs from its pin")
	}
	hash := Digest(data)
	if hash != spec.SHA256 {
		return nil, fmt.Errorf("prepared input hash mismatch: got %s", hash)
	}
	result.Evidence = &Evidence{Kind: spec.Kind, SourceSHA256: Digest(source), SHA256: hash, Operations: len(contracts)}
	result.contracts = contracts
	// Unique adjacent files preserve relative refs without sharing cleanup ownership
	// between concurrent measurements of the same source.
	file, err := os.CreateTemp(filepath.Dir(input), ".sdkgen-prepared-"+hash+"-*.json")
	if err != nil {
		return nil, err
	}
	result.Path = file.Name()
	result.created = true
	if _, err = file.Write(data); err != nil {
		file.Close()
		result.Close()
		return nil, err
	}
	if err = file.Close(); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}
func (prepared *Prepared) Close() error {
	if !prepared.created {
		return nil
	}
	prepared.created = false
	return os.Remove(prepared.Path)
}

// Bundle expands authoring-only Operation refs and standard reusable objects,
// then rebases schema refs. It preserves schema content rather than repairing it.
// Exact source pins are verified by the corpus runner before this function runs.
func Bundle(input string) ([]byte, []operationContract, error) {
	absolute, err := filepath.Abs(input)
	if err != nil {
		return nil, nil, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, nil, err
	}
	state := &bundler{root: filepath.Dir(absolute), entry: absolute, active: map[string]bool{}}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return nil, nil, err
	}
	var raw any
	if err = yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, err
	}
	value, err := state.walk(raw, nil, absolute)
	if err != nil {
		return nil, nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}
	contracts, err := state.contracts(value)
	return append(encoded, '\n'), contracts, err
}
func (state *bundler) walk(value any, path []string, source string) (any, error) {
	switch object := value.(type) {
	case map[string]any:
		context := openapiwalk.ObjectContextAt(path)
		reusable := context == openapiwalk.ObjectOperation || context == openapiwalk.ObjectResponse || context == openapiwalk.ObjectHeader || context == openapiwalk.ObjectParameter || context == openapiwalk.ObjectRequestBody || context == openapiwalk.ObjectLink || context == openapiwalk.ObjectCallback
		if reusable {
			if reference, _ := object["$ref"].(string); reference != "" {
				if len(object) != 1 {
					return nil, fmt.Errorf("reusable reference with siblings requires an explicit provider preparation rule")
				}
				target, file, key, err := state.load(source, reference)
				if err != nil {
					return nil, err
				}
				if state.active[key] {
					return nil, fmt.Errorf("cyclic operation authoring reference")
				}
				state.active[key] = true
				resolved, err := state.walk(target, path, file)
				delete(state.active, key)
				return resolved, err
			}
		}
		result := make(map[string]any, len(object))
		for key, child := range object {
			if openapiwalk.ReferenceChildOpaque(path, key, child) {
				result[key] = child
				continue
			}
			mapping := len(path) >= 2 && path[len(path)-1] == "mapping" && path[len(path)-2] == "discriminator" && openapiwalk.ObjectContextAt(path[:len(path)-2]) == openapiwalk.ObjectSchema
			mappingRef := false
			if text, ok := child.(string); ok {
				mappingRef = mapping && (strings.ContainsAny(text, "/.#:") || strings.HasPrefix(text, "#"))
			}
			if (key == "$ref" || key == "operationRef" || mappingRef) && source != state.entry {
				reference, ok := child.(string)
				if !ok {
					return nil, fmt.Errorf("non-string author reference")
				}
				parsed, err := url.Parse(reference)
				if err != nil {
					return nil, err
				}
				if !parsed.IsAbs() {
					file := source
					if parsed.Path != "" {
						file = filepath.Join(filepath.Dir(source), filepath.FromSlash(parsed.Path))
					}
					relative, err := filepath.Rel(state.root, file)
					if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
						return nil, fmt.Errorf("author reference escapes input directory")
					}
					parsed.Path = filepath.ToSlash(relative)
					if mappingRef && !strings.HasPrefix(parsed.Path, ".") {
						parsed.Path = "./" + parsed.Path
					}
					parsed.RawPath = ""
					result[key] = parsed.String()
					continue
				}
			}
			if key == "$id" && source != state.entry {
				return nil, fmt.Errorf("schema resource IDs in operation authoring files require a provider-specific bundle")
			}
			normalized, err := state.walk(child, append(path, key), source)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case []any:
		result := make([]any, len(object))
		for index, child := range object {
			normalized, err := state.walk(child, append(path, strconv.Itoa(index)), source)
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	default:
		return value, nil
	}
}
func (state *bundler) load(source, reference string) (any, string, string, error) {
	parsed, err := url.Parse(reference)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || filepath.IsAbs(parsed.Path) {
		return nil, "", "", fmt.Errorf("author operation reference must be a local file reference")
	}
	file := source
	if parsed.Path != "" {
		file = filepath.Join(filepath.Dir(source), filepath.FromSlash(parsed.Path))
	}
	file, err = filepath.EvalSymlinks(file)
	if err != nil {
		return nil, "", "", err
	}
	relative, err := filepath.Rel(state.root, file)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, "", "", fmt.Errorf("author reference escapes input directory")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, "", "", err
	}
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, "", "", err
	}
	value, err := pointer(root, parsed.Fragment)
	return value, file, file + "#" + parsed.Fragment, err
}
func pointer(value any, fragment string) (any, error) {
	if fragment == "" {
		return value, nil
	}
	if !strings.HasPrefix(fragment, "/") {
		return nil, fmt.Errorf("author reference requires a JSON Pointer")
	}
	for _, part := range strings.Split(fragment[1:], "/") {
		for index := 0; index < len(part); index++ {
			if part[index] == '~' {
				if index+1 >= len(part) || (part[index+1] != '0' && part[index+1] != '1') {
					return nil, fmt.Errorf("invalid JSON Pointer escape")
				}
				index++
			}
		}
		token := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch object := value.(type) {
		case map[string]any:
			var exists bool
			value, exists = object[token]
			if !exists {
				return nil, fmt.Errorf("author reference pointer is missing")
			}
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(object) || strconv.Itoa(index) != token {
				return nil, fmt.Errorf("author reference array pointer is invalid")
			}
			value = object[index]
		default:
			return nil, fmt.Errorf("author reference pointer crosses a scalar")
		}
	}
	return value, nil
}
func (state *bundler) resolve(value any) (map[string]any, error) {
	if state.document != nil {
		return containedObject(state.document, value)
	}
	object, _ := value.(map[string]any)
	seen := map[string]bool{}
	for {
		reference, _ := object["$ref"].(string)
		if reference == "" {
			return object, nil
		}
		if seen[reference] {
			return nil, fmt.Errorf("cyclic reusable contract")
		}
		seen[reference] = true
		target, source, _, err := state.load(state.entry, reference)
		if err != nil {
			return nil, err
		}
		normalized, err := state.walk(target, nil, source)
		if err != nil {
			return nil, err
		}
		object, _ = normalized.(map[string]any)
	}
}
func (state *bundler) contracts(value any) ([]operationContract, error) {
	root, _ := value.(map[string]any)
	paths, _ := root["paths"].(map[string]any)
	var contracts []operationContract
	for path, raw := range paths {
		item, _ := raw.(map[string]any)
		for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"} {
			operation, _ := item[method].(map[string]any)
			if operation == nil {
				continue
			}
			id, _ := operation["operationId"].(string)
			contract := operationContract{Route: strings.ToUpper(method) + " " + path, ID: id, Body: operation["requestBody"] != nil}
			for _, owner := range []map[string]any{item, operation} {
				parameters, _ := owner["parameters"].([]any)
				for _, raw := range parameters {
					parameter, err := state.resolve(raw)
					if err != nil {
						return nil, err
					}
					name, _ := parameter["name"].(string)
					location, _ := parameter["in"].(string)
					// OpenAPI requires these header declarations to be ignored.
					if location == "header" && (strings.EqualFold(name, "Accept") || strings.EqualFold(name, "Content-Type") || strings.EqualFold(name, "Authorization")) {
						continue
					}
					required, _ := parameter["required"].(bool)
					next := parameterContract{Name: name, Location: location, Required: required}
					replaced := false
					for index, prior := range contract.Parameters {
						if prior.Name == name && prior.Location == location {
							contract.Parameters[index] = next
							replaced = true
							break
						}
					}
					if !replaced {
						contract.Parameters = append(contract.Parameters, next)
					}
				}
			}
			responses, _ := operation["responses"].(map[string]any)
			for status, raw := range responses {
				if strings.HasPrefix(status, "x-") {
					continue
				}
				response, err := state.resolve(raw)
				if err != nil {
					return nil, err
				}
				content, _ := response["content"].(map[string]any)
				if len(content) == 0 {
					contract.Responses = append(contract.Responses, responseContract{Status: status})
				}
				for media, raw := range content {
					representation, _ := raw.(map[string]any)
					schema, err := state.resolve(representation["schema"])
					if err != nil {
						return nil, err
					}
					kind, _ := schema["type"].(string)
					contract.Responses = append(contract.Responses, responseContract{Status: status, Media: media, Schema: representation["schema"] != nil, Type: kind})
				}
			}
			contracts = append(contracts, contract)
		}
	}
	sort.Slice(contracts, func(i, j int) bool { return contracts[i].Route < contracts[j].Route })
	return contracts, nil
}

// Verify checks the concrete contract fields whose absence can otherwise still
// produce compilable SDKs. It does not claim arbitrary schema equivalence.
func (prepared *Prepared) Verify(document *ir.Document) error {
	if prepared.Evidence == nil {
		return nil
	}
	prepared.Evidence.ContractsVerified = false
	operations := map[string]ir.Operation{}
	for _, operation := range document.Operations {
		operations[operation.RouteKey] = operation
	}
	for _, expected := range prepared.contracts {
		operation, exists := operations[expected.Route]
		if !exists || operation.OperationID != expected.ID {
			return fmt.Errorf("prepared operation identity lost: %s", expected.Route)
		}
		if expected.Body && operation.RequestBody == nil {
			return fmt.Errorf("prepared request body lost: %s", expected.Route)
		}
		for _, parameter := range expected.Parameters {
			found := false
			for _, actual := range operation.Parameters {
				if actual.Name == parameter.Name && actual.Location == parameter.Location && actual.Required == parameter.Required {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("prepared parameter contract lost: %s %s", expected.Route, parameter.Name)
			}
		}
		for _, response := range expected.Responses {
			found := false
			for _, actual := range operation.Responses {
				if actual.Status != response.Status {
					continue
				}
				if response.Media == "" {
					found = true
					break
				}
				for _, media := range actual.Content {
					if media.ContentType != response.Media {
						continue
					}
					schema, _ := media.Schema.(map[string]any)
					if reference, _ := schema["$ref"].(string); strings.HasPrefix(reference, "#/") {
						fragment, decodeErr := url.PathUnescape(strings.TrimPrefix(reference, "#"))
						resolved, err := pointer(document.Raw, fragment)
						if err == nil && decodeErr == nil {
							schema, _ = resolved.(map[string]any)
						}
					}
					kind, _ := schema["type"].(string)
					if (!response.Schema || media.Schema != nil) && (response.Type == "" || response.Type == kind) {
						found = true
						break
					}
				}
			}
			if !found {
				return fmt.Errorf("prepared response contract lost: %s %s %s", expected.Route, response.Status, response.Media)
			}
		}
	}
	prepared.Evidence.ContractsVerified = true
	return nil
}
