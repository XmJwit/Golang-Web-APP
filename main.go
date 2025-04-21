package main

import (
	"encoding/json"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type APIConfig struct {
	gorm.Model
	Name         string `gorm:"unique"` // 配置名称
	URL          string // 目标API地址
	Method       string // GET/POST
	Parameters   string // JSON格式参数
	RequestBody  string // JSON格式请求体
	Headers      string // JSON格式请求头
	ResponseRule string // 响应解析规则
}

var DB *gorm.DB

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("api_config.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// 自动迁移表结构
	DB.AutoMigrate(&APIConfig{})
}

func main() {
	InitDB()

	r := gin.Default()

	// 1. 设置模板解析（用于HTML页面）
	// 开发模式禁用缓存
	if gin.Mode() == gin.DebugMode {
		absPath, _ := filepath.Abs("templates")
    	r.LoadHTMLGlob(filepath.Join(absPath, "/*.html"))
	} else {
		// 生产环境可以缓存模板
		fs := os.DirFS("templates")
		tmpl := template.Must(template.ParseFS(fs, "*.html"))
		r.SetHTMLTemplate(tmpl)
	}

	// 2. 设置静态文件服务
	// 静态文件服务（确保路径正确）
	r.Static("/static", "./static")

	// 3. 页面路由
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "API配置列表",
		})
	})

	r.GET("/detail", func(c *gin.Context) {
		c.HTML(http.StatusOK, "detail.html", gin.H{
			"title": "配置详情",
		})
	})

	r.GET("/edit", func(c *gin.Context) {
		title := "添加配置"
		if c.Query("id") != "" {
			title = "编辑配置"
		}
		c.HTML(http.StatusOK, "edit.html", gin.H{
			"title": title,
		})
	})

	r.GET("/upload", func(c *gin.Context) {
		c.HTML(http.StatusOK, "upload.html", gin.H{
			"title": "文件上传",
		})
	})
	// 4. API路由组（与前端对接）
	api := r.Group("/api")
	{
		api.GET("/list", listConfigs)     // 分页列表
		api.GET("/get", getConfig)        // 获取详情
		api.POST("/add", addConfig)       // 添加配置
		api.POST("/edit", editConfig)     // 更新配置
		api.POST("/delete", deleteConfig) // 删除配置
		api.POST("/upload", uploadFile)   // 文件上传
	}

	r.Run(":8080")
}

// 分页请求参数
type ListRequest struct {
	Page      int    `form:"page" binding:"min=1"`                                    // 页码，最小为1
	PageSize  int    `form:"pageSize" binding:"min=1,max=100"`                        // 每页数量，1-100
	Keyword   string `form:"keyword"`                                                 // 名称/URL关键词
	Method    string `form:"method"`                                                  // 按方法过滤
	SortField string `form:"sortField" binding:"oneof=id name created_at updated_at"` // 排序字段
	SortOrder string `form:"sortOrder" binding:"oneof=asc desc"`                      // 排序方向
}

// 最大每页数量限制
const MaxPageSize = 100

// 允许的排序字段
var allowedSortFields = map[string]bool{
	"id":         true,
	"name":       true,
	"created_at": true,
	"updated_at": true,
}

// 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// 分页查询响应结构
type PaginationResponse struct {
	Total       int64       `json:"total"`       // 总记录数
	TotalPages  int         `json:"totalPages"`  // 总页数
	CurrentPage int         `json:"currentPage"` // 当前页码
	PageSize    int         `json:"pageSize"`    // 每页数量
	Data        []APIConfig `json:"data"`        // 当前页数据
}

// 添加配置
func addConfig(c *gin.Context) {
	var config APIConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, Response{400, "参数错误", nil})
		return
	}

	if result := DB.Create(&config); result.Error != nil {
		c.JSON(500, Response{500, "创建失败", nil})
		return
	}

	c.JSON(200, Response{0, "success", config})
}

// 修改配置
func editConfig(c *gin.Context) {
	var input struct {
		ID           uint   `json:"id" binding:"required"`
		Name         string `json:"name"`
		URL          string `json:"url"`
		Method       string `json:"method"`
		Parameters   string `json:"parameters"`
		RequestBody  string `json:"requestBody"`
		Headers      string `json:"headers"`
		ResponseRule string `json:"responseRule"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, Response{400, "参数错误", nil})
		return
	}

	// 查找现有配置
	var existing APIConfig
	if err := DB.First(&existing, input.ID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, Response{404, "配置不存在", nil})
			return
		}
		c.JSON(500, Response{500, "查询失败", nil})
		return
	}

	// 更新字段
	updates := map[string]interface{}{
		"Name":         input.Name,
		"URL":          input.URL,
		"Method":       input.Method,
		"Parameters":   input.Parameters,
		"RequestBody":  input.RequestBody,
		"Headers":      input.Headers,
		"ResponseRule": input.ResponseRule,
	}

	if err := DB.Model(&existing).Updates(updates).Error; err != nil {
		c.JSON(500, Response{500, "更新失败", nil})
		return
	}

	c.JSON(200, Response{0, "success", existing})
}

// 删除配置
func deleteConfig(c *gin.Context) {
	var input struct {
		ID uint `json:"id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, Response{400, "参数错误", nil})
		return
	}

	// 物理删除记录
	result := DB.Unscoped().Delete(&APIConfig{}, input.ID)
	if result.Error != nil {
		c.JSON(500, Response{500, "删除失败", nil})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(404, Response{404, "配置不存在", nil})
		return
	}

	c.JSON(200, Response{0, "success", nil})
}

// 获取配置详情
func getConfig(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, Response{400, "ID参数错误", nil})
		return
	}

	var config APIConfig
	if err := DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, Response{404, "配置不存在", nil})
			return
		}
		c.JSON(500, Response{500, "查询失败", nil})
		return
	}

	c.JSON(200, Response{0, "success", config})
}

// 分页查询配置列表
func listConfigs(c *gin.Context) {
	var req ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(400, Response{400, "参数错误", nil})
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 || req.PageSize > MaxPageSize {
		req.PageSize = 10
	}
	if req.SortField == "" {
		req.SortField = "id" // 默认按ID排序
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc" // 默认降序
	}

	query := DB.Model(&APIConfig{})

	// 关键词搜索（名称或URL）
	if req.Keyword != "" {
		keyword := "%" + strings.ToLower(req.Keyword) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(url) LIKE ?", keyword, keyword)
	}

	// 方法过滤
	if req.Method != "" {
		query = query.Where("method = ?", strings.ToUpper(req.Method))
	}

	// 安全排序（防止SQL注入）
	sortField := req.SortField
	if !allowedSortFields[sortField] {
		sortField = "id"
	}
	sortOrder := strings.ToUpper(req.SortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}
	orderClause := sortField + " " + sortOrder

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(500, Response{500, "查询失败", nil})
		return
	}

	// 计算分页参数
	offset := (req.Page - 1) * req.PageSize
	totalPages := int(math.Ceil(float64(total) / float64(req.PageSize)))

	// 查询数据（带排序）
	var configs []APIConfig
	err := query.Order(orderClause).
		Offset(offset).
		Limit(req.PageSize).
		Find(&configs).Error

	if err != nil {
		c.JSON(500, Response{500, "查询失败", nil})
		return
	}

	// 返回结果
	c.JSON(200, Response{
		Code:    0,
		Message: "success",
		Data: PaginationResponse{
			Total:       total,
			TotalPages:  totalPages,
			CurrentPage: req.Page,
			PageSize:    req.PageSize,
			Data:        configs,
		},
	})
}

// 文件上传处理
func uploadFile(c *gin.Context) {
	configIDStr := c.PostForm("configId")
	configID, err := strconv.Atoi(configIDStr)
	if err != nil {
		c.JSON(400, Response{400, "configId参数错误", nil})
		return
	}

	// 获取配置
	var config APIConfig
	if err := DB.First(&config, configID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, Response{404, "配置不存在", nil})
			return
		}
		c.JSON(500, Response{500, "查询失败", nil})
		return
	}

	// 处理文件上传
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, Response{400, "文件上传失败", nil})
		return
	}

	// 保存临时文件
	tempFile, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		c.JSON(500, Response{500, "临时文件创建失败", nil})
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if err := c.SaveUploadedFile(file, tempFile.Name()); err != nil {
		c.JSON(500, Response{500, "文件保存失败", nil})
		return
	}

	// 调用目标API
	respData, err := callRemoteAPI(config, tempFile)
	if err != nil {
		c.JSON(502, Response{502, "远程API调用失败", nil})
		return
	}

	// 解析响应
	fileURL, err := parseResponse(config.ResponseRule, respData)
	if err != nil {
		c.JSON(502, Response{502, "响应解析失败", nil})
		return
	}

	c.JSON(200, Response{0, "success", gin.H{"url": fileURL}})
}

// 调用远程API的辅助函数
func callRemoteAPI(config APIConfig, file *os.File) ([]byte, error) {
	// 实现文件上传逻辑
	// 根据config.Method决定使用POST或GET
	// 添加headers、parameters等
	// 使用http.Client发送请求
	// 返回响应内容

	// 示例实现：
	client := &http.Client{}
	req, _ := http.NewRequest(config.Method, config.URL, nil)

	// 添加headers
	var headers map[string]string
	json.Unmarshal([]byte(config.Headers), &headers)
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	// 处理文件上传（需要根据实际API要求实现）
	// 可能需要使用multipart/form-data格式

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// 解析响应
func parseResponse(rule string, response []byte) (string, error) {
	// 实现响应解析逻辑
	// 示例：假设规则是json字段路径
	// 或者正则表达式匹配

	// 示例：直接返回整个响应（实际需要根据规则解析）
	return string(response), nil
}
