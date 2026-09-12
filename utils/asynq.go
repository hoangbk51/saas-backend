package utils

import "github.com/hibiken/asynq"

// AsynqClient là biến toàn cục dùng để Enqueue Job ở bất kỳ đâu trong dự án
var AsynqClient *asynq.Client
