package golang

import (
	"fmt"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func (planner *modelPlanner) nullableLiterals(name string, node *schemaplan.Node, inner modelType, innerPlan modelPlan) ([]modelLiteral, error) {
	literals, _, err := planner.literalMembers(name, node)
	if err != nil {
		return nil, err
	}
	for index, literal := range literals {
		if literal.JSON == "null" {
			literals[index].Expression = fmt.Sprintf("%s(types.Null[%s]())", name, inner.Type)
			continue
		}
		found := false
		for _, candidate := range innerPlan.Literals {
			if candidate.JSON != literal.JSON {
				continue
			}
			expression := candidate.Name
			if !innerPlan.ScalarLiteral {
				expression += "()"
			}
			literals[index].Expression = fmt.Sprintf("%s(types.NonNull(%s))", name, expression)
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("missing non-null literal projection for %s", name)
		}
	}
	return literals, nil
}
