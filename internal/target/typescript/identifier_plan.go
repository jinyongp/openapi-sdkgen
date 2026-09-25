package typescript

import (
	"fmt"
	"sort"
	"strconv"
)

// localIdentifierKey preserves the exact target fields, not a delimiter-joined
// approximation or an emitted name. Different artifacts may reuse every name.
type localIdentifierKey struct {
	role      localIdentifierRole
	identity  string
	qualifier string
}

type localIdentifierRole string

const (
	localTypeImport      localIdentifierRole = "t"
	localResourceBuilder localIdentifierRole = "r"
	localLinkGroup       localIdentifierRole = "l"
)

// localIdentifierPlan is owned by one emitted module, including its nested
// functions. Requests are idempotent uses of a binding. Fixed declarations and
// readable/protected names are reserved before the allocation is frozen.
// Nothing here is shared across generations or stored in the semantic IR.
type localIdentifierPlan struct {
	owner    string
	requests map[localIdentifierKey]struct{}
	reserved map[string]struct{}
	names    map[localIdentifierKey]string
	frozen   bool
}

func newLocalIdentifierPlan(owner string) *localIdentifierPlan {
	return &localIdentifierPlan{
		owner: owner, requests: make(map[localIdentifierKey]struct{}),
		reserved: make(map[string]struct{}),
	}
}

func (plan *localIdentifierPlan) reserve(names ...string) error {
	if err := plan.requireCollecting(); err != nil {
		return err
	}
	for _, name := range names {
		if name == "" {
			return fmt.Errorf("private identifiers for %q: empty reserved binding", plan.owner)
		}
	}
	for _, name := range names {
		plan.reserved[name] = struct{}{}
	}
	return nil
}

func (plan *localIdentifierPlan) request(key localIdentifierKey) error {
	if err := plan.requireCollecting(); err != nil {
		return err
	}
	switch key.role {
	case localTypeImport, localResourceBuilder, localLinkGroup:
	default:
		return fmt.Errorf("private identifiers for %q: unsupported local role %q", plan.owner, key.role)
	}
	plan.requests[key] = struct{}{}
	return nil
}

func (plan *localIdentifierPlan) requireCollecting() error {
	if plan == nil || plan.owner == "" {
		return fmt.Errorf("private identifier allocation requires an artifact owner")
	}
	if plan.frozen {
		return fmt.Errorf("private identifiers for %q are already frozen", plan.owner)
	}
	return nil
}

func (plan *localIdentifierPlan) freeze() error {
	if err := plan.requireCollecting(); err != nil {
		return err
	}
	keys := make([]localIdentifierKey, 0, len(plan.requests))
	for key := range plan.requests {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, right := keys[i], keys[j]
		if left.role != right.role {
			return left.role < right.role
		}
		if left.identity != right.identity {
			return left.identity < right.identity
		}
		return left.qualifier < right.qualifier
	})
	names := make(map[localIdentifierKey]string, len(keys))
	occupied := make(map[string]struct{}, len(plan.reserved)+len(keys))
	for name := range plan.reserved {
		occupied[name] = struct{}{}
	}
	next := make(map[localIdentifierRole]uint64)
	for _, key := range keys {
		prefix := "__sdkgen_" + string(key.role) + "_d"
		for {
			ordinal := next[key.role]
			next[key.role]++
			name := prefix + strconv.FormatUint(ordinal, 36)
			if _, used := occupied[name]; used {
				continue
			}
			names[key] = name
			occupied[name] = struct{}{}
			break
		}
	}
	plan.names = names
	plan.frozen = true
	return nil
}

func (plan *localIdentifierPlan) resolve(key localIdentifierKey) (string, error) {
	if plan == nil || !plan.frozen {
		return "", fmt.Errorf("private identifier reference requires a frozen artifact plan")
	}
	name, exists := plan.names[key]
	if !exists {
		return "", fmt.Errorf("private identifiers for %q: undeclared %s binding (%q, %q)", plan.owner, key.role, key.identity, key.qualifier)
	}
	return name, nil
}

func typeImportIdentifierKey(modulePath, exportName string) localIdentifierKey {
	return localIdentifierKey{role: localTypeImport, identity: modulePath, qualifier: exportName}
}

func resourceBuilderIdentifierKey(identity string) localIdentifierKey {
	return localIdentifierKey{role: localResourceBuilder, identity: identity}
}

func linkGroupIdentifierKey(group generatedLinkGroup) localIdentifierKey {
	return localIdentifierKey{role: localLinkGroup, identity: operationRouteKey(group.SourceOperation), qualifier: group.Name}
}
