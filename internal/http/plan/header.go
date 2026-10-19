package plan

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
)

func HeaderSchema(document *ir.Document, header map[string]any) (any, string, error) {
	content, _ := header["content"].(map[string]any)
	if len(content) == 0 {
		return header["schema"], "", nil
	}
	if len(content) != 1 {
		return nil, "", fmt.Errorf("Header Object content must define exactly one media type")
	}
	for contentType, value := range content {
		media, _ := value.(map[string]any)
		media, err := ResolveComponentObject(document, media, "mediaTypes")
		if err != nil {
			return nil, "", err
		}
		return media["schema"], contentType, nil
	}
	panic("unreachable nonempty header content")
}
