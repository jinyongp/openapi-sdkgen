package golang

import (
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
)

func prepareResourceNames(names *namePlan, operations []ir.Operation, modelNamesUsed map[string]bool) error {
	identities := map[string]string{"/": "ResourceRoot"}
	parameters := make(map[string]string)
	children := make(map[string]map[string]string)
	selectors := make(map[string]string)
	calls := make(map[string]map[string]string)
	for _, operation := range operations {
		identity, raw := "", "Resource"
		supported := true
		for _, part := range httpplan.ResourcePathParts(operation.Path) {
			parent := identity
			if parent == "" {
				parent = "/"
			}
			name, parameter, supported := httpplan.ResourcePathPart(part)
			if !supported {
				break
			}
			if parameter {
				if _, exists := selectors[parent]; !exists {
					selectors[parent] = "By" + resourceParameterName(name)
				}
				identity += "/{}"
				raw += "By" + resourceParameterName(name)
				if _, exists := parameters[identity]; !exists {
					parameters[identity] = raw + "Parameter"
				}
			} else {
				if children[parent] == nil {
					children[parent] = make(map[string]string)
				}
				children[parent][part] = strings.TrimSuffix(parent, "/") + "/" + part
				identity += "/" + part
				raw += exportedName(part)
			}
			if _, exists := identities[identity]; !exists {
				identities[identity] = raw
			}
		}
		// Mixed parameter/literal segments have only a Routes entry.
		for _, part := range httpplan.ResourcePathParts(operation.Path) {
			_, _, ok := httpplan.ResourcePathPart(part)
			supported = supported && ok
		}
		if supported {
			if identity == "" {
				identity = "/"
			}
			terminal, err := httpplan.ResourceTerminal(operation, httpplan.ResourcePathParts(operation.Path))
			if err != nil {
				// Literal namespaces have a Go fallback name even when the shared
				// identifier has no words. Keep the HTTP method as the terminal.
				terminal, err = httpplan.ResourceMethodTerminal(operation.Method)
				if err != nil {
					if operation.Method == "" {
						return err
					}
					terminal = exportedName(operation.Method)
				}
			}
			if calls[identity] == nil {
				calls[identity] = make(map[string]string)
			}
			calls[identity][operation.RouteKey] = terminal
		}
	}
	names.resourceMembers = make(map[string]map[string]string, len(identities))
	for _, identity := range sortedKeys(identities) {
		members := make(map[string]string)
		used := make(map[string]bool)
		if identity == "/" {
			for _, name := range []string{"Operations", "Routes", "Links", "ResourceRoot"} {
				used[name] = true
			}
		}
		reserve := func(key, raw, owner string) error {
			name, err := reservePublicName(used, raw, owner, "")
			if err == nil {
				members[key] = name
			}
			return err
		}
		for _, literal := range sortedKeys(children[identity]) {
			if err := reserve("child:"+literal, literal, children[identity][literal]); err != nil {
				return err
			}
		}
		if raw := selectors[identity]; raw != "" {
			if err := reserve("selector", raw, strings.TrimSuffix(identity, "/")+"/{}/selector"); err != nil {
				return err
			}
		}
		for _, route := range sortedKeys(calls[identity]) {
			if err := reserve("call:"+route, calls[identity][route], route+"/resource"); err != nil {
				return err
			}
		}
		names.resourceMembers[identity] = members
	}
	names.resources = make(map[string]string, len(identities))
	names.resourceParameters = make(map[string]string, len(parameters))
	used := make(map[string]bool)
	for _, identity := range sortedKeys(identities) {
		name, err := reservePublicName(used, identities[identity], identity, "")
		if err != nil {
			return err
		}
		names.resources[identity] = name
	}
	for _, identity := range sortedKeys(parameters) {
		name, err := reserveDerivedName(modelNamesUsed, parameters[identity], identity, modelRootNames)
		if err != nil {
			return err
		}
		names.resourceParameters[identity] = name
	}
	return nil
}

func resourceParameterName(name string) string {
	if len(name) >= 2 && strings.EqualFold(name[len(name)-2:], "id") {
		return "ID"
	}
	return exportedName(name)
}
