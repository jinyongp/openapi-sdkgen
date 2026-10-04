package typescript

// callableInputBinding is selected from prepared input facts, never generated
// source. Both the operation import and runtime closure use this owner.
func callableInputBinding(item ManifestOperation) (name, template string) {
	if len(item.InputSections) == 0 {
		return "bindNoInputOperation", "operation-bind-none.ts"
	}
	if item.prepared.inputRequired {
		return "bindInputOperation", "operation-bind-input.ts"
	}
	return "bindOptionalInputOperation", "operation-bind-optional.ts"
}
