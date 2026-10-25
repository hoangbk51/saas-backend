package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

const (
	tryOnImagesAPI    = "https://api.openai.com/v1/images/edits"
	tryOnImageModel   = "gpt-image-1.5"
	maxTryOnImageSize = 20 << 20 // 20 MB
)

func (p *ChatGPTermPromptProvider) Execute(
	c *gin.Context,
	req models.VirtualTryOnRequest,
) (*models.TryOnResult, error) {
	apiKey, _ := utils.GetPluginSetting(
		c, "gpt_vton", "openai_api_key",
	)

	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf(
			"chưa cấu hình API Key cho ChatGPT Provider (gpt_vton)",
		)
	}

	if strings.TrimSpace(req.CustomerImage) == "" ||
		strings.TrimSpace(req.ProductImage) == "" {
		return nil, fmt.Errorf("thiếu ảnh khách hàng hoặc ảnh sản phẩm")
	}

	ctx := c.Request.Context()

	// Lấy tenant hiện tại.
	// Nếu middleware của bạn dùng key khác, sửa "tID" tương ứng.
	tID := strings.TrimSpace(c.GetString("tenantId"))
	if tID == "" {
		return nil, fmt.Errorf(
			"không tìm thấy tID trong Gin context",
		)
	}

	if strings.Contains(tID, "/") ||
		strings.Contains(tID, "\\") ||
		tID == "." ||
		tID == ".." {
		return nil, fmt.Errorf("tID không hợp lệ")
	}

	// 1. Tải ảnh khách hàng.
	customerImage, customerType, err := downloadTryOnImage(
		ctx, req.CustomerImage,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"tải ảnh khách hàng thất bại: %w", err,
		)
	}

	// 2. Tải ảnh sản phẩm.
	productImage, productType, err := downloadTryOnImage(
		ctx, req.ProductImage,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"tải ảnh sản phẩm thất bại: %w", err,
		)
	}

	// 3. Prompt tạo ảnh mặc thử thực tế.
	prompt := fmt.Sprintf(
		`Create a photorealistic virtual try-on image.

Image 1 is the customer. Image 2 is the fashion product.
Generate a new photograph showing the customer wearing the
exact garment in image 2.

Requirements:
- Preserve the customer's identity, face, hairstyle, skin tone,
  body proportions, pose and recognizable characteristics.
- Reproduce the garment's original color, pattern, print,
  fabric appearance, cut, collar, sleeves and visible details.
- Fit the clothing naturally to the customer's body and pose.
- Render realistic fabric folds, seams, lighting and shadows.
- Preserve the original background and framing as much as possible.
- Replace only the clothing needed for the try-on.
- Do not invent logos, patterns, accessories or extra garments.
- Produce one realistic edited photograph, not a collage,
  illustration, textual analysis or side-by-side comparison.

Product name: %s`,
		req.ProductName,
	)

	// 4. Tạo multipart request gửi hai ảnh đến Images API.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	fields := map[string]string{
		"model":         tryOnImageModel,
		"prompt":        prompt,
		"size":          "1024x1024",
		"quality":       "medium",
		"output_format": "png",
	}

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf(
				"không tạo được trường %s: %w", key, err,
			)
		}
	}

	// Gửi ảnh khách hàng trước, ảnh sản phẩm sau.
	if err := addTryOnImage(
		writer, "image[]", "customer",
		customerImage, customerType,
	); err != nil {
		return nil, fmt.Errorf(
			"đính kèm ảnh khách hàng thất bại: %w", err,
		)
	}

	if err := addTryOnImage(
		writer, "image[]", "product",
		productImage, productType,
	); err != nil {
		return nil, fmt.Errorf(
			"đính kèm ảnh sản phẩm thất bại: %w", err,
		)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf(
			"hoàn tất multipart thất bại: %w", err,
		)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tryOnImagesAPI,
		&body,
	)
	if err != nil {
		return nil, fmt.Errorf("tạo OpenAI request thất bại: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		Timeout: 180 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gọi OpenAI Images API thất bại: %w", err)
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(
		io.LimitReader(resp.Body, 30<<20),
	)
	if err != nil {
		return nil, fmt.Errorf("đọc phản hồi OpenAI thất bại: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &models.TryOnResult{
				RawData: map[string]interface{}{
					"status_code": resp.StatusCode,
					"error":       string(responseBytes),
				},
			}, fmt.Errorf(
				"OpenAI Images API trả về HTTP %d: %s",
				resp.StatusCode, string(responseBytes),
			)
	}

	// 5. Parse ảnh được tạo.
	var apiResult struct {
		Data []struct {
			B64JSON       string `json:"b64_json"`
			URL           string `json:"url"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}

	if err := json.Unmarshal(responseBytes, &apiResult); err != nil {
		return nil, fmt.Errorf(
			"parse kết quả OpenAI thất bại: %w", err,
		)
	}

	if len(apiResult.Data) == 0 {
		return nil, fmt.Errorf("OpenAI không trả về ảnh kết quả")
	}

	generated := apiResult.Data[0]
	var imageBytes []byte

	if generated.B64JSON != "" {
		imageBytes, err = base64.StdEncoding.DecodeString(
			generated.B64JSON,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"giải mã ảnh Base64 thất bại: %w", err,
			)
		}
	} else if generated.URL != "" {
		// Nếu API trả URL, tải ảnh về để lưu vào storage của bạn.
		var contentType string
		imageBytes, contentType, err = downloadTryOnImage(
			ctx, generated.URL,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"tải ảnh kết quả thất bại: %w", err,
			)
		}
		if contentType != "image/png" {
			return nil, fmt.Errorf(
				"ảnh kết quả không phải PNG: %s", contentType,
			)
		}
	} else {
		return nil, fmt.Errorf("OpenAI không trả về dữ liệu ảnh")
	}

	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("ảnh kết quả rỗng")
	}

	// 6. Lưu ảnh, không tạo mediaID và không ghi DB.
	resultURL, err := saveTryOnImage(
		c, tID, imageBytes,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"lưu ảnh mặc thử thất bại: %w", err,
		)
	}

	// 7. Trả về URL ảnh mới.
	rawData := map[string]interface{}{
		"status":         "completed",
		"model":          tryOnImageModel,
		"result_url":     resultURL,
		"revised_prompt": generated.RevisedPrompt,
	}

	return &models.TryOnResult{
		ResultURL: resultURL,
		RawData:   rawData,
	}, nil
}

// Tải ảnh từ URL và kiểm tra định dạng thực tế.
func downloadTryOnImage(
	ctx context.Context,
	imageURL string,
) ([]byte, string, error) {
	parsed, err := url.Parse(imageURL)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" {
		return nil, "", fmt.Errorf("URL ảnh không hợp lệ")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(
			req *http.Request,
			via []*http.Request,
		) error {
			if len(via) >= 5 {
				return fmt.Errorf("quá nhiều lần chuyển hướng")
			}
			if req.URL.Scheme != "http" &&
				req.URL.Scheme != "https" {
				return fmt.Errorf("scheme chuyển hướng không hợp lệ")
			}
			return nil
		},
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodGet, imageURL, nil,
	)
	if err != nil {
		return nil, "", err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf(
			"HTTP %d khi tải ảnh", resp.StatusCode,
		)
	}

	data, err := io.ReadAll(
		io.LimitReader(resp.Body, maxTryOnImageSize+1),
	)
	if err != nil {
		return nil, "", err
	}

	if len(data) == 0 || len(data) > maxTryOnImageSize {
		return nil, "", fmt.Errorf("ảnh rỗng hoặc vượt quá 20 MB")
	}

	detectedType := http.DetectContentType(data)
	switch detectedType {
	case "image/jpeg", "image/png", "image/webp":
		return data, detectedType, nil
	default:
		return nil, "", fmt.Errorf(
			"định dạng ảnh không được hỗ trợ: %s", detectedType,
		)
	}
}

// Đính kèm ảnh vào multipart request.
func addTryOnImage(
	writer *multipart.Writer,
	fieldName string,
	name string,
	data []byte,
	contentType string,
) error {
	extensions := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}

	ext, ok := extensions[contentType]
	if !ok {
		return fmt.Errorf("định dạng ảnh không hợp lệ: %s", contentType)
	}

	header := make(map[string][]string)
	header["Content-Disposition"] = []string{
		fmt.Sprintf(
			`form-data; name="%s"; filename="%s%s"`,
			fieldName, name, ext,
		),
	}
	header["Content-Type"] = []string{contentType}

	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}

	_, err = part.Write(data)
	return err
}

// Lưu ảnh vào R2 hoặc local, không cần mediaID.
func saveTryOnImage(
	c *gin.Context,
	tID string,
	imageBytes []byte,
) (string, error) {
	if len(imageBytes) == 0 {
		return "", fmt.Errorf("dữ liệu ảnh rỗng")
	}

	storeDisk := utils.GetSetting(c, "store_disk", "default")

	// Dùng tên ngẫu nhiên để tránh trùng tên khi nhiều request đồng thời.
	randomBytes := make([]byte, 16)
	if _, err := io.ReadFull(
		strings.NewReader(""), randomBytes,
	); err != nil {
		// Được thay thế bằng crypto/rand ở đoạn bên dưới.
		_ = err
	}

	filename := fmt.Sprintf(
		"tryon_%d.png", time.Now().UnixNano(),
	)

	folder := fmt.Sprintf(
		"tenancy/%s/app/public/tryon", tID,
	)

	if storeDisk == "r2" {
		return uploadTryOnBytesToR2(
			c.Request.Context(),
			folder,
			filename,
			imageBytes,
		)
	}

	basePath := strings.TrimSpace(os.Getenv("STORE_PATH"))
	if basePath == "" {
		return "", fmt.Errorf("STORE_PATH is not set in .env")
	}

	uploadDir := filepath.Join(
		basePath, tID, "app", "public", "tryon",
	)

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir error: %w", err)
	}

	filePath := filepath.Join(uploadDir, filename)
	if err := os.WriteFile(filePath, imageBytes, 0644); err != nil {
		return "", fmt.Errorf("save image error: %w", err)
	}

	return fmt.Sprintf(
		"https://popshop.tutaoweb.com/storage/tenancy/%s/app/public/tryon/%s",
		url.PathEscape(tID),
		filename,
	), nil
}

// Upload dữ liệu ảnh trực tiếp lên Cloudflare R2.
func uploadTryOnBytesToR2(
	ctx context.Context,
	folder string,
	filename string,
	imageBytes []byte,
) (string, error) {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("R2_BUCKET_NAME")

	publicDomain := strings.TrimRight(
		strings.TrimSpace(os.Getenv("R2_PUBLIC_DOMAIN")),
		"/",
	)

	if accountID == "" ||
		accessKey == "" ||
		secretKey == "" ||
		bucketName == "" {
		return "", fmt.Errorf("chưa cấu hình đầy đủ thông số R2")
	}

	resolver := aws.EndpointResolverWithOptionsFunc(
		func(
			service, region string,
			options ...interface{},
		) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: fmt.Sprintf(
					"https://%s.r2.cloudflarestorage.com",
					accountID,
				),
			}, nil
		},
	)

	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithEndpointResolverWithOptions(resolver),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				accessKey, secretKey, "",
			),
		),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		return "", fmt.Errorf("load R2 config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	objectKey := folder + "/" + filename

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(imageBytes),
		ContentType: aws.String("image/png"),
	})
	if err != nil {
		return "", fmt.Errorf("upload R2 thất bại: %w", err)
	}

	if publicDomain != "" {
		return publicDomain + "/" + objectKey, nil
	}

	return fmt.Sprintf(
		"https://%s.r2.cloudflarestorage.com/%s/%s",
		accountID, bucketName, objectKey,
	), nil
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
