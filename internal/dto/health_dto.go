package dto

type HealthResponse struct {
	Message     string `json:"message"`
	Timestamp   string `json:"timestamp"`
	DBStatus    string `json:"db_status"`
	RedisStatus string `json:"redis_status"`
}
