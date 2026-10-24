package response

import "github.com/gin-gonic/gin"

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func Success(ctx *gin.Context, statusCode int, message string, data interface{}) {
	response := Response{
		Success: true,
		Message: message,
		Data:    data,
	}
	ctx.JSON(statusCode, response)
}

func Error(ctx *gin.Context, statusCode int, message string, err string) {
	response := Response{
		Success: false,
		Message: message,
		Error:   err,
	}
	ctx.JSON(statusCode, response)
}
