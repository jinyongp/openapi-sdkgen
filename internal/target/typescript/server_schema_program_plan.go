package typescript

import "fmt"

func prepareServerSchemaRuntimePlan(source *sourcePlan) error {
	if source.serverSchemaPrograms != nil {
		return nil
	}
	runtime := newSchemaRuntimePlan()
	runtime.views = true
	runtime.kindComponents = make(map[string]map[projection]map[string]bool)
	for _, root := range source.runtimeFeatures.inbound {
		for _, feature := range root.features {
			switch string(feature) {
			case "schema.allOf", "schema.oneOf", "schema.anyOf", "schema.not", "schema.if", "schema.contains", "schema.dependentSchemas", "schema.unevaluatedProperties", "schema.unevaluatedItems", "schema.patternProperties":
				runtime.annotations = true
			}
		}
	}
	callbackFacts := make(map[string]executionSchemaFacts)
	webhookFacts := make(map[string]executionSchemaFacts)
	facts := map[string]executionSchemaFacts{"callback": {references: make(map[executionSchemaReference]bool)}, "webhook": {references: make(map[executionSchemaReference]bool)}}
	for _, definition := range source.callbacks {
		callbackFacts[callbackIdentity(definition)] = definition.runtimeFacts
		for reference := range definition.runtimeFacts.references {
			facts["callback"].references[reference] = true
		}
	}
	for _, definition := range source.webhooks {
		key := definition.name + "\x00" + definition.method
		webhookFacts[key] = definition.runtimeFacts
		for reference := range definition.runtimeFacts.references {
			facts["webhook"].references[reference] = true
		}
	}
	planner := newExecutionPlanner(source.document)
	for _, kind := range []string{"callback", "webhook"} {
		_, input, output, err := planner.schemaClosure(facts[kind])
		if err != nil {
			return err
		}
		names := map[projection]map[string]bool{projectionInput: {}, projectionOutput: {}}
		for _, name := range input {
			names[projectionInput][name] = true
		}
		for _, name := range output {
			names[projectionOutput][name] = true
		}
		runtime.kindComponents[kind] = names
		wire := newWireRenderContext(wirePropertiesConstructed)
		wire.semanticOnly = true
		wire.schemaPrograms = runtime
		for _, direction := range []projection{projectionInput, projectionOutput} {
			for _, name := range sortedStringKeys(names[direction]) {
				if _, err := wire.wireSchemaDescriptorForDocument(source.document, componentSchemaValue(source.document, name), direction); err != nil {
					return err
				}
			}
		}
	}
	callbacks, callbackFailures := collectCallbacksDiagnostics(source.document, source.omittedOperations, runtime)
	webhooks, webhookFailures := collectWebhooksDiagnostics(source.document, runtime)
	if len(callbackFailures) > 0 {
		return callbackFailures[0]
	}
	if len(webhookFailures) > 0 {
		return webhookFailures[0]
	}
	for index := range callbacks {
		key := callbackIdentity(callbacks[index])
		original, exists := callbackFacts[key]
		if !exists {
			return fmt.Errorf("unprepared callback schema owner %q", key)
		}
		callbacks[index].runtimeFacts = original
	}
	for index := range webhooks {
		key := webhooks[index].name + "\x00" + webhooks[index].method
		original, exists := webhookFacts[key]
		if !exists {
			return fmt.Errorf("unprepared webhook schema owner %q", key)
		}
		webhooks[index].runtimeFacts = original
	}
	source.callbacks = callbacks
	source.webhooks = webhooks
	if err := runtime.freeze(); err != nil {
		return err
	}
	source.serverSchemaPrograms = runtime
	return nil
}
