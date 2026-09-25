package typescript

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
)

// aggregateEntityKey is a lossless semantic key. The canonical payload is never
// replaced by its digest in a map or a reference; hashes choose spelling only.
type aggregateEntityKey struct {
	domain  string
	payload string
}

func newAggregateEntity(domain string, fields ...string) aggregateEntityKey {
	payload := append([]byte(nil), "sdkgen.lexical.v1"...)
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(domain)))
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(fields)))
	payload = append(payload, domain...)
	for _, field := range fields {
		payload = binary.BigEndian.AppendUint64(payload, uint64(len(field)))
		payload = append(payload, field...)
	}
	return aggregateEntityKey{domain: domain, payload: string(payload)}
}

type aggregateIdentifierRole string

const (
	aggregateBaseFactory        aggregateIdentifierRole = "bf"
	aggregatePaginationFactory  aggregateIdentifierRole = "pf"
	aggregateLinksFactory       aggregateIdentifierRole = "lf"
	aggregateStreamFactory      aggregateIdentifierRole = "sf"
	aggregateOperationValue     aggregateIdentifierRole = "ov"
	aggregateBaseValue          aggregateIdentifierRole = "bv"
	aggregatePaginationValue    aggregateIdentifierRole = "pv"
	aggregateLinksValue         aggregateIdentifierRole = "lv"
	aggregateStreamValue        aggregateIdentifierRole = "sv"
	aggregateInputWire          aggregateIdentifierRole = "wi"
	aggregateOutputWire         aggregateIdentifierRole = "wo"
	aggregateEnumValues         aggregateIdentifierRole = "ev"
	aggregateEnumRecord         aggregateIdentifierRole = "e"
	aggregateCallbackType       aggregateIdentifierRole = "ct"
	aggregateCallbackDefinition aggregateIdentifierRole = "cd"
	aggregateCallbackEndpoint   aggregateIdentifierRole = "ce"
	aggregateWebhookType        aggregateIdentifierRole = "wt"
	aggregateWebhookDefinition  aggregateIdentifierRole = "wd"
)

// Footprints include the actual declarations derived from an emitted stem, not
// only that stem. In particular a reserved ...Context or ...Handlers must also
// force an extension. These static tags are independent of enum ordering.
func aggregateRoleFootprint(role aggregateIdentifierRole) (domain string, suffixes []string, valid bool) {
	switch role {
	case aggregateBaseFactory, aggregatePaginationFactory, aggregateLinksFactory, aggregateStreamFactory,
		aggregateOperationValue, aggregateBaseValue, aggregatePaginationValue, aggregateLinksValue, aggregateStreamValue:
		return "route", []string{""}, true
	case aggregateInputWire, aggregateOutputWire, aggregateEnumValues, aggregateEnumRecord:
		return "schema", []string{""}, true
	case aggregateCallbackType:
		return "callback", []string{"Context", "Response"}, true
	case aggregateCallbackDefinition, aggregateCallbackEndpoint:
		return "callback", []string{""}, true
	case aggregateWebhookType:
		return "webhook", []string{"Context", "Response"}, true
	case aggregateWebhookDefinition:
		return "webhook", []string{"", "Handlers", "PathParameters"}, true
	default:
		return "", nil, false
	}
}

type aggregateBindingKey struct {
	entity aggregateEntityKey
	role   aggregateIdentifierRole
}

type aggregateIdentifierPlan struct {
	owner    string
	requests map[aggregateEntityKey]map[aggregateIdentifierRole]struct{}
	reserved map[string]struct{}
	names    map[aggregateBindingKey]string
	frozen   bool
	digest   func([]byte) [sha256.Size]byte // Instance-local injection for forced-collision tests.
}

func newAggregateIdentifierPlan(owner string) *aggregateIdentifierPlan {
	return &aggregateIdentifierPlan{owner: owner,
		requests: make(map[aggregateEntityKey]map[aggregateIdentifierRole]struct{}),
		reserved: make(map[string]struct{}), digest: sha256.Sum256}
}

func (plan *aggregateIdentifierPlan) requireCollecting() error {
	if plan == nil || plan.owner == "" {
		return fmt.Errorf("aggregate identifiers require an artifact owner")
	}
	if plan.frozen {
		return fmt.Errorf("aggregate identifiers for %q are already frozen", plan.owner)
	}
	return nil
}

func (plan *aggregateIdentifierPlan) reserve(names ...string) error {
	if err := plan.requireCollecting(); err != nil {
		return err
	}
	for _, name := range names {
		if name == "" {
			return fmt.Errorf("aggregate identifiers for %q: empty reserved binding", plan.owner)
		}
	}
	for _, name := range names {
		plan.reserved[name] = struct{}{}
	}
	return nil
}

func (plan *aggregateIdentifierPlan) request(entity aggregateEntityKey, roles ...aggregateIdentifierRole) error {
	if err := plan.requireCollecting(); err != nil {
		return err
	}
	if entity.payload == "" || len(roles) == 0 {
		return fmt.Errorf("aggregate identifiers for %q: incomplete entity or roles", plan.owner)
	}
	for _, role := range roles {
		domain, _, valid := aggregateRoleFootprint(role)
		if !valid || domain != entity.domain {
			return fmt.Errorf("aggregate identifiers for %q: role %q cannot own domain %q", plan.owner, role, entity.domain)
		}
	}
	set := plan.requests[entity]
	if set == nil {
		set = make(map[aggregateIdentifierRole]struct{})
		plan.requests[entity] = set
	}
	for _, role := range roles {
		set[role] = struct{}{}
	}
	return nil
}

var aggregateBase32 = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

const aggregateMinimumPrefix = 12

type aggregateDigestGroup struct {
	digest   string
	entities []aggregateEntityKey
}

func (plan *aggregateIdentifierPlan) freeze() error {
	if err := plan.requireCollecting(); err != nil {
		return err
	}
	if plan.digest == nil {
		return fmt.Errorf("aggregate identifiers for %q have no digest function", plan.owner)
	}
	byDigest := make(map[string][]aggregateEntityKey, len(plan.requests))
	for entity := range plan.requests {
		digest := plan.digest([]byte(entity.payload))
		encoded := aggregateBase32.EncodeToString(digest[:])
		byDigest[encoded] = append(byDigest[encoded], entity)
	}
	groups := make([]aggregateDigestGroup, 0, len(byDigest))
	for digest, entities := range byDigest {
		sort.Slice(entities, func(i, j int) bool { return entities[i].payload < entities[j].payload })
		groups = append(groups, aggregateDigestGroup{digest: digest, entities: entities})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].digest < groups[j].digest })
	occupied := make(map[string]struct{}, len(plan.reserved))
	for name := range plan.reserved {
		occupied[name] = struct{}{}
	}
	names := make(map[aggregateBindingKey]string)
	for index, group := range groups {
		width := aggregateMinimumPrefix
		if index > 0 {
			width = max(width, aggregateCommonPrefix(group.digest, groups[index-1].digest)+1)
		}
		if index+1 < len(groups) {
			width = max(width, aggregateCommonPrefix(group.digest, groups[index+1].digest)+1)
		}
		for _, entity := range group.entities {
			roles := make([]aggregateIdentifierRole, 0, len(plan.requests[entity]))
			for role := range plan.requests[entity] {
				roles = append(roles, role)
			}
			sort.Slice(roles, func(i, j int) bool { return roles[i] < roles[j] })
			token := ""
			if len(group.entities) == 1 {
				for length := width; length <= len(group.digest); length++ {
					candidate := "h" + group.digest[:length]
					if aggregateTokenAvailable(candidate, roles, occupied) {
						token = candidate
						break
					}
				}
			}
			if token == "" {
				base := "x" + aggregateBase32.EncodeToString([]byte(entity.payload))
				token = base
				for retry := uint64(0); !aggregateTokenAvailable(token, roles, occupied); retry++ {
					token = base + "_r" + strconv.FormatUint(retry, 36)
				}
			}
			for _, role := range roles {
				stem := aggregateIdentifierStem(role, token)
				_, suffixes, _ := aggregateRoleFootprint(role)
				for _, suffix := range suffixes {
					name := stem + suffix
					if _, exists := occupied[name]; exists {
						return fmt.Errorf("aggregate identifiers for %q: duplicate final binding %q", plan.owner, name)
					}
					occupied[name] = struct{}{}
				}
				names[aggregateBindingKey{entity: entity, role: role}] = stem
			}
		}
	}
	plan.names = names
	plan.frozen = true
	return nil
}

func aggregateIdentifierStem(role aggregateIdentifierRole, token string) string {
	return "__sdkgen_" + string(role) + "_" + token
}

func aggregateTokenAvailable(token string, roles []aggregateIdentifierRole, occupied map[string]struct{}) bool {
	for _, role := range roles {
		stem := aggregateIdentifierStem(role, token)
		_, suffixes, _ := aggregateRoleFootprint(role)
		for _, suffix := range suffixes {
			if _, exists := occupied[stem+suffix]; exists {
				return false
			}
		}
	}
	return true
}

func aggregateCommonPrefix(left, right string) int {
	index := 0
	for index < len(left) && index < len(right) && left[index] == right[index] {
		index++
	}
	return index
}

func (plan *aggregateIdentifierPlan) resolve(entity aggregateEntityKey, role aggregateIdentifierRole) (string, error) {
	if plan == nil || !plan.frozen {
		return "", fmt.Errorf("aggregate reference requires a frozen artifact owner")
	}
	name, exists := plan.names[aggregateBindingKey{entity: entity, role: role}]
	if !exists {
		return "", fmt.Errorf("aggregate identifiers for %q: undeclared %s binding for %q", plan.owner, role, entity.payload)
	}
	return name, nil
}
