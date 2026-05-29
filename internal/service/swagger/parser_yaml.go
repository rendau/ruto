package swagger

import "gopkg.in/yaml.v3"

func parseDocumentYAML(body []byte) (document, error) {
	var raw document
	if err := yaml.Unmarshal(body, &raw); err != nil {
		return document{}, err
	}
	return raw, nil
}
