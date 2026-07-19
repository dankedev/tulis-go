package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// apiCall makes an HTTP request to the Tulis API with the API key header.
func apiCall(method, path string, body any) ([]byte, error) {
	url := strings.TrimRight(srv.apiURL, "/") + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal error: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", srv.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func textResult(text string) toolResult {
	return toolResult{
		Content: []toolContent{{Type: "text", Text: text}},
	}
}

func callListPosts(args json.RawMessage) toolResult {
	var params map[string]any
	json.Unmarshal(args, &params)

	qp := []string{}
	if v, ok := params["status"]; ok && v != nil {
		qp = append(qp, fmt.Sprintf("status=%s", v))
	}
	if v, ok := params["post_type"]; ok && v != nil {
		qp = append(qp, fmt.Sprintf("type=%s", v))
	}
	if v, ok := params["search"]; ok && v != nil {
		qp = append(qp, fmt.Sprintf("search=%s", v))
	}
	if v, ok := params["page"]; ok && v != nil {
		qp = append(qp, fmt.Sprintf("page=%v", v))
	} else {
		qp = append(qp, "page=1")
	}
	if v, ok := params["limit"]; ok && v != nil {
		qp = append(qp, fmt.Sprintf("per_page=%v", v))
	} else {
		qp = append(qp, "per_page=20")
	}

	path := "/posts"
	if len(qp) > 0 {
		path += "?" + strings.Join(qp, "&")
	}

	data, err := apiCall("GET", path, nil)
	if err != nil {
		return textResult(fmt.Sprintf("Error: %v", err))
	}

	var pretty bytes.Buffer
	json.Indent(&pretty, data, "", "  ")
	return textResult(pretty.String())
}

func callGetPost(args json.RawMessage) toolResult {
	var params map[string]any
	json.Unmarshal(args, &params)

	idOrSlug, ok := params["id_or_slug"].(string)
	if !ok || idOrSlug == "" {
		return textResult("Error: id_or_slug is required")
	}

	data, err := apiCall("GET", "/posts/"+idOrSlug, nil)
	if err != nil {
		return textResult(fmt.Sprintf("Error: %v", err))
	}

	var pretty bytes.Buffer
	json.Indent(&pretty, data, "", "  ")
	return textResult(pretty.String())
}

func callCreatePost(args json.RawMessage) toolResult {
	var params map[string]any
	json.Unmarshal(args, &params)

	if params["status"] == nil || params["status"] == "" {
		params["status"] = "draft"
	}
	if params["post_type"] == nil || params["post_type"] == "" {
		params["post_type"] = "post"
	}

	// Convert taxonomy_ids to string array
	if v, ok := params["taxonomy_ids"]; ok {
		if arr, ok := v.([]any); ok {
			strs := make([]string, len(arr))
			for i, item := range arr {
				strs[i] = fmt.Sprint(item)
			}
			params["taxonomy_ids"] = strs
		}
	}

	data, err := apiCall("POST", "/posts", params)
	if err != nil {
		return textResult(fmt.Sprintf("Error: %v", err))
	}

	var pretty bytes.Buffer
	json.Indent(&pretty, data, "", "  ")
	return textResult(pretty.String())
}

func callUpdatePost(args json.RawMessage) toolResult {
	var params map[string]any
	json.Unmarshal(args, &params)

	id, ok := params["id"].(string)
	if !ok || id == "" {
		return textResult("Error: id is required")
	}
	delete(params, "id")

	data, err := apiCall("PUT", "/posts/"+id, params)
	if err != nil {
		return textResult(fmt.Sprintf("Error: %v", err))
	}

	var pretty bytes.Buffer
	json.Indent(&pretty, data, "", "  ")
	return textResult(pretty.String())
}

func callListTaxonomies(args json.RawMessage) toolResult {
	var params map[string]any
	json.Unmarshal(args, &params)

	path := "/taxonomies"
	if v, ok := params["type"]; ok && v != nil && v != "" {
		path += "?type=" + fmt.Sprint(v)
	}

	data, err := apiCall("GET", path, nil)
	if err != nil {
		return textResult(fmt.Sprintf("Error: %v", err))
	}

	var pretty bytes.Buffer
	json.Indent(&pretty, data, "", "  ")
	return textResult(pretty.String())
}

func callCreateTaxonomy(args json.RawMessage) toolResult {
	var params map[string]any
	json.Unmarshal(args, &params)

	data, err := apiCall("POST", "/taxonomies", params)
	if err != nil {
		return textResult(fmt.Sprintf("Error: %v", err))
	}

	var pretty bytes.Buffer
	json.Indent(&pretty, data, "", "  ")
	return textResult(pretty.String())
}

func callUploadMedia(args json.RawMessage) toolResult {
	var params map[string]any
	json.Unmarshal(args, &params)

	data, err := apiCall("POST", "/media/upload-via-url", params)
	if err != nil {
		return textResult(fmt.Sprintf("Error: %v", err))
	}

	var pretty bytes.Buffer
	json.Indent(&pretty, data, "", "  ")
	return textResult(pretty.String())
}
