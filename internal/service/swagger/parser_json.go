package swagger

import "encoding/json"

func parseDocumentJSON(body []byte) (document, error) {
	var raw document
	if err := json.Unmarshal(body, &raw); err != nil {
		return document{}, err
	}
	return raw, nil
}
