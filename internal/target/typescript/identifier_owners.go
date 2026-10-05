package typescript

// operationLocalIdentifiers reserves declarations from every contributing
// fragment before schema-import aliases are collected. Link group requests join
// the same owner; public leaf names remain protected independently of compaction.
func operationLocalIdentifiers(module operationModulePlan, item ManifestOperation, links []generatedLink) (*localIdentifierPlan, error) {
	names := newLocalIdentifierPlan(module.path)
	if err := names.reserve(
		"bindOperation", "bindStreamOperation", "RequestFunction", "WireSchemas",
		"TransportError", "BinaryBody", "OperationStream", "RawResponseFor", "RequestOptions", "StreamSource",
		"OperationTypeIdentity", "LinkCalls", "OperationRawCall", "PaginateCall", "ResourceRawCapability",
		"RouteTypeIdentity", "OperationPublicType", "OperationResourceRawCapability", "ResourceRawMethod", "ResourceMethod",
		"bindInputOperation", "bindNoInputOperation", "bindOptionalInputOperation",
		"RouteInput", "RouteOptions", "RouteOutput", "RouteRawResponse", "RouteResourceInput", "StreamCall",
		"ContractSchemas", "Errors", "createPaginator", "PaginateInput", "mergeLinkInput", "resolveLinkInput",
		"LinkInvocation", "RequiredLinkInvocation", "APIError", "RouteKey", "RequestInputs", "Input", "ResourceInput",
		"Options", "Output", "Error", "RawResponse", "BaseCall", "RawCall", "ResourceBaseCall", "ResourceRawCall",
		"HTTPError", "HTTPErrorFor", "HTTPErrorIdentity", "HTTPStatusRange",
		"Pagination", "Links", "Stream", "ResourceStream", "ExactCall", "ResourceCall", "Contract", "LinkTargets", "LinkInvoker", "invoke",
		"bindBase", "bindPagination", "bindLinks", "bindStream", "request", "inputSchemas", "outputSchemas",
		"base", "input", "requestOptions", "response", "invocation", "targets", "__sdkgen_Properties",
	); err != nil {
		return nil, err
	}
	for _, suffix := range []string{
		"Options", "PathInput", "QueryInput", "QuerystringInput", "HeaderInput", "CookieInput", "BodyInput",
		"FilterInput", "SortInput", "Input", "ResourceInput", "Output", "RawResponse", "RawCall", "Call",
		"ResourceRawCall", "ResourceCall",
	} {
		if err := names.reserve(operationLocalTypePrefix + suffix); err != nil {
			return nil, err
		}
	}
	for _, binding := range item.prepared.pathBindings {
		if err := names.reserve(binding); err != nil {
			return nil, err
		}
	}
	for _, link := range links {
		binding, err := generatedLinkVariableName(link)
		if err != nil {
			return nil, err
		}
		if err := names.reserve(binding); err != nil {
			return nil, err
		}
	}
	for _, group := range linkGroupsForSource(links, module.routeKey) {
		if err := names.request(linkGroupIdentifierKey(group)); err != nil {
			return nil, err
		}
	}
	return names, nil
}
