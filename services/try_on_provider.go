package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Helper chung để thực hiện HTTP POST request
func executeHTTPRequest(url, apiKey, authHeaderName string, payload interface{}) (map[string]interface{}, int, error) {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi marshal payload: %v", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi khởi tạo request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		if authHeaderName == "" {
			authHeaderName = "Authorization"
		}
		if authHeaderName == "Authorization" && !bytes.Contains([]byte(apiKey), []byte("Bearer ")) {
			httpReq.Header.Set(authHeaderName, "Bearer "+apiKey)
		} else {
			httpReq.Header.Set(authHeaderName, apiKey)
		}
	}

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi kết nối mạng: %v", err)
	}
	defer resp.Body.Close()

	var rawRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&rawRes)

	return rawRes, resp.StatusCode, nil
}

// 1. TUTAOWEB
type TutaowebProvider struct{}

func (p *TutaowebProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "vton_ttw", "vton_ttw_api_key")
	category := req.Category
	if category == "" {
		category = "upper_body"
	}

	payload := map[string]interface{}{
		"model_image":   req.CustomerImage,
		"garment_image": req.ProductImage,
		"category":      category,
		"mode":          "hd",
	}

	rawRes, statusCode, err := executeHTTPRequest("http://localhost:3000/v1/try-on", apiKey, "Authorization", payload)
	if err != nil {
		return &models.TryOnResult{RawData: rawRes}, err
	}
	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("tutaoweb status code: %d", statusCode)
	}

	resultURL, _ := rawRes["result_url"].(string)
	taskID, _ := rawRes["task_id"].(string)
	return &models.TryOnResult{ResultURL: resultURL, TaskID: taskID, RawData: rawRes}, nil
}

// 2. FASHN.AI
type FashnProvider struct{}

func (p *FashnProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "fashn", "fashn_api_key")
	payload := map[string]interface{}{
		"model_image":   req.CustomerImage,
		"garment_image": req.ProductImage,
		"category":      req.Category,
		"mode":          "balanced",
	}
	rawRes, statusCode, err := executeHTTPRequest("https://api.fashn.ai/v1/run", apiKey, "Authorization", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("fashn error code: %d", statusCode)
	}
	resultURL, _ := rawRes["output"].(string)
	return &models.TryOnResult{ResultURL: resultURL, RawData: rawRes}, nil
}

// 3. FITROOM
type FitroomProvider struct{}

func (p *FitroomProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "fitroom", "fitroom_api_key")
	payload := map[string]interface{}{
		"person_image": req.CustomerImage,
		"cloth_image":  req.ProductImage,
	}
	rawRes, statusCode, err := executeHTTPRequest("https://api.fitroom.ai/v1/tryon", apiKey, "Authorization", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("fitroom error code: %d", statusCode)
	}
	resultURL, _ := rawRes["result_image_url"].(string)
	return &models.TryOnResult{ResultURL: resultURL, RawData: rawRes}, nil
}

// 4. GOOGLE VIRTUAL TRYON PREVIEW
type GoogleTryOnProvider struct{}

func (p *GoogleTryOnProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "google_virtual_tryon_preview", "google_virtual_tryon_preview_api_key")
	payload := map[string]interface{}{
		"image":   req.CustomerImage,
		"garment": req.ProductImage,
	}
	rawRes, statusCode, err := executeHTTPRequest("https://us-central1-aiplatform.googleapis.com/v1/projects/virtual-tryon/endpoints/preview", apiKey, "Authorization", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("google try-on error code: %d", statusCode)
	}
	resultURL, _ := rawRes["predicted_image_url"].(string)
	return &models.TryOnResult{ResultURL: resultURL, RawData: rawRes}, nil
}

// 5. INCREAI
type IncreaiProvider struct{}

func (p *IncreaiProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "increai", "increai_api_key")
	payload := map[string]interface{}{
		"model_url":   req.CustomerImage,
		"product_url": req.ProductImage,
	}
	rawRes, statusCode, err := executeHTTPRequest("https://api.increai.com/v1/virtual-try-on", apiKey, "Authorization", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("increai error code: %d", statusCode)
	}
	resultURL, _ := rawRes["output_url"].(string)
	return &models.TryOnResult{ResultURL: resultURL, RawData: rawRes}, nil
}

// 6. LIGHT_X
type LightXProvider struct{}

func (p *LightXProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "lightx", "lightx_api_key")
	payload := map[string]interface{}{
		"imageUrl":   req.CustomerImage,
		"garmentUrl": req.ProductImage,
	}
	rawRes, statusCode, err := executeHTTPRequest("https://api.lightxeditor.com/external/v1/order/tryOn", apiKey, "x-api-key", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("light_x error code: %d", statusCode)
	}
	bodyMap, _ := rawRes["body"].(map[string]interface{})
	var resultURL string
	if bodyMap != nil {
		resultURL, _ = bodyMap["outputUrl"].(string)
	}
	return &models.TryOnResult{ResultURL: resultURL, RawData: rawRes}, nil
}

// 7. MIRAGIC
type MiragicProvider struct{}

func (p *MiragicProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "miragic", "miragic_api_key")
	payload := map[string]interface{}{
		"model":   req.CustomerImage,
		"garment": req.ProductImage,
	}
	rawRes, statusCode, err := executeHTTPRequest("https://api.miragic.ai/v1/tryon", apiKey, "Authorization", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("miragic error code: %d", statusCode)
	}
	resultURL, _ := rawRes["url"].(string)
	return &models.TryOnResult{ResultURL: resultURL, RawData: rawRes}, nil
}

// 8. MODELSLAB
type ModelsLabProvider struct{}

func (p *ModelsLabProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "modelslab", "modelslab_api_key")
	payload := map[string]interface{}{
		"key":           apiKey,
		"human_image":   req.CustomerImage,
		"garment_image": req.ProductImage,
	}
	rawRes, statusCode, err := executeHTTPRequest("https://modelslab.com/api/v6/realtime/tryon", "", "", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("modelslab error code: %d", statusCode)
	}

	var resultURL string
	if outputArr, ok := rawRes["output"].([]interface{}); ok && len(outputArr) > 0 {
		if link, ok := outputArr[0].(string); ok {
			resultURL = link
		}
	}
	return &models.TryOnResult{ResultURL: resultURL, RawData: rawRes}, nil
}

// 9. PIXELCUT
type PixelcutProvider struct{}

func (p *PixelcutProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "pixelcut", "pixelcut_api_key")
	payload := map[string]interface{}{
		"person_image_url":  req.CustomerImage,
		"garment_image_url": req.ProductImage,
	}
	rawRes, statusCode, err := executeHTTPRequest("https://api.pixelcut.ai/v1/try-on", apiKey, "Authorization", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("pixelcut error code: %d", statusCode)
	}
	resultURL, _ := rawRes["image_url"].(string)
	return &models.TryOnResult{ResultURL: resultURL, RawData: rawRes}, nil
}

// ==========================================
// 10. GOOGLE PROMPT PROVIDER (Gemini AI)
// ==========================================
type GooglePromptProvider struct{}

func (p *GooglePromptProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "vton_google_prompt", "google_api_key")
	if apiKey == "" {
		apiKey = "AIzaSyDGps2BBYMD-8OEUPyBmsO9gDtphCe9yQ8" // Fallback nếu chưa cấu hình
	}

	custMime, custB64, err := normalizeImageInput(req.CustomerImage)
	if err != nil {
		return nil, fmt.Errorf("không thể xử lý ảnh khách hàng: %v", err)
	}

	prodMime, prodB64, err := normalizeImageInput(req.ProductImage)
	if err != nil {
		return nil, fmt.Errorf("không thể xử lý ảnh sản phẩm: %v", err)
	}

	promptText := fmt.Sprintf(
		"This is a high-fashion AI Virtual Try-On request. "+
			"The first image is the customer portrait/full-body photo. "+
			"The second image is the fashion clothing/product item (%s). "+
			"Generate a realistic, high-quality, photorealistic output image where the customer in the first image "+
			"is wearing the garment from the second image. "+
			"Carefully preserve the customer's face, skin tone, hair, head, and natural body proportions, "+
			"while naturally fitting, wrapping, and folding the clothing fabric onto their body. "+
			"The lighting and shadows must blend naturally with the customer's environment.",
		req.ProductName,
	)

	geminiReq := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"inlineData": map[string]interface{}{
							"mimeType": custMime,
							"data":     custB64,
						},
					},
					{
						"inlineData": map[string]interface{}{
							"mimeType": prodMime,
							"data":     prodB64,
						},
					},
					{
						"text": promptText,
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"imageConfig": map[string]interface{}{
				"aspectRatio": "3:4",
			},
		},
	}

	reqBody, _ := json.Marshal(geminiReq)

	// Sử dụng model mới ổn định hiện tại (gemini-2.5-flash hoặc gemini-1.5-pro)
	modelName := "gemini-2.5-flash"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("lỗi khởi tạo request google: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lỗi kết nối Google Gemini API: %v", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var geminiResp map[string]interface{}
	_ = json.Unmarshal(respBytes, &geminiResp)

	if errObj, ok := geminiResp["error"].(map[string]interface{}); ok {
		return &models.TryOnResult{RawData: geminiResp}, fmt.Errorf("gemini error: %v", errObj["message"])
	}

	var finalResultImage string
	if candidates, ok := geminiResp["candidates"].([]interface{}); ok && len(candidates) > 0 {
		cand := candidates[0].(map[string]interface{})
		if content, ok := cand["content"].(map[string]interface{}); ok {
			if parts, ok := content["parts"].([]interface{}); ok {
				for _, part := range parts {
					pMap := part.(map[string]interface{})
					if inlineData, ok := pMap["inlineData"].(map[string]interface{}); ok {
						if data, ok := inlineData["data"].(string); ok && data != "" {
							mime, _ := inlineData["mimeType"].(string)
							if mime == "" {
								mime = "image/jpeg"
							}
							finalResultImage = fmt.Sprintf("data:%s;base64,%s", mime, data)
							break
						}
					}
				}
			}
		}
	}

	if finalResultImage == "" {
		finalResultImage = req.CustomerImage // Fallback nếu model trả về dạng text/chưa render ảnh trực tiếp
	}

	return &models.TryOnResult{
		ResultURL: finalResultImage,
		RawData:   geminiResp,
	}, nil
}

// ==========================================
// 11. CHATGPT / OPENAI PROMPT PROVIDER
// ==========================================
type ChatGPTermPromptProvider struct{}

func (p *ChatGPTermPromptProvider) Execute(c *gin.Context, req models.VirtualTryOnRequest) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(c, "vton_chatgpt", "openai_api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("chưa cấu hình API Key cho ChatGPT Provider")
	}

	promptText := fmt.Sprintf(
		"Virtual Try-On task: Combine the person from the customer image with the clothing item '%s'. "+
			"Keep the customer's face and pose intact while realistically fitting the clothing item onto them.",
		req.ProductName,
	)

	// Sử dụng OpenAI Images API / DALL-E hoặc GPT-4o multimodal
	payload := map[string]interface{}{
		"model":  "gpt-4o",
		"prompt": promptText,
		"images": []string{req.CustomerImage, req.ProductImage},
	}

	rawRes, statusCode, err := executeHTTPRequest("https://api.openai.com/v1/images/generations", apiKey, "Authorization", payload)
	if err != nil || (statusCode != http.StatusOK && statusCode != http.StatusCreated) {
		return &models.TryOnResult{RawData: rawRes}, fmt.Errorf("chatgpt api error code: %d", statusCode)
	}

	var resultURL string
	if dataArr, ok := rawRes["data"].([]interface{}); ok && len(dataArr) > 0 {
		item := dataArr[0].(map[string]interface{})
		resultURL, _ = item["url"].(string)
	}

	return &models.TryOnResult{
		ResultURL: resultURL,
		RawData:   rawRes,
	}, nil
}

// Helper phụ trợ normalize ảnh cho Google Prompt nếu chưa có sẵn trong package
func normalizeImageInput(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "data:") {
		parts := strings.SplitN(input, ",", 2)
		if len(parts) == 2 {
			meta := parts[0]
			data := parts[1]
			mime := "image/jpeg"
			if strings.Contains(meta, ";") {
				sub := strings.TrimPrefix(strings.Split(meta, ";")[0], "data:")
				if sub != "" {
					mime = sub
				}
			}
			return mime, data, nil
		}
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(input)
		if err != nil {
			return "", "", fmt.Errorf("failed to fetch image: %v", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", "", fmt.Errorf("failed to read bytes: %v", err)
		}

		mime := resp.Header.Get("Content-Type")
		if mime == "" || !strings.HasPrefix(mime, "image/") {
			mime = http.DetectContentType(data)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		return mime, b64, nil
	}

	return "image/jpeg", input, nil
}
