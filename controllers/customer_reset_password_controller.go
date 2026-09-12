package controllers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"go-saas/tasks"
	"go-saas/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func generateSecureToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func ForgotPasswordHandler(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email không hợp lệ"})
		return
	}

	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi kết nối DB"})
		return
	}

	// 1. Kiểm tra Customer có tồn tại không
	var customer struct {
		ID        uint64
		FirstName string
		LastName  string
	}

	query := `SELECT id, first_name, last_name FROM customers WHERE email = ? AND active = 1 AND deleted_at IS NULL LIMIT 1`
	err = tenantDB.QueryRow(query, req.Email).Scan(&customer.ID, &customer.FirstName, &customer.LastName)

	// Bảo mật: Dù email không tồn tại vẫn trả về thông báo thành công (tránh Enumeration Attack)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"message": "Nếu email tồn tại trong hệ thống, chúng tôi đã gửi hướng dẫn đặt lại mật khẩu."})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống"})
		return
	}

	// 2. Tạo Reset Token & Lưu vào DB (Hạn 30 phút)
	resetToken := generateSecureToken()
	expiresAt := time.Now().Add(30 * time.Minute)

	updateQuery := `
		UPDATE customers 
		SET reset_password_token = ?, 
		    reset_password_expires_at = ? 
		WHERE id = ?
	`
	_, err = tenantDB.Exec(updateQuery, resetToken, expiresAt, customer.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tạo mã khôi phục"})
		return
	}

	// 3. Đẩy Asynq Job gửi Email
	if utils.AsynqClient != nil {
		storeName := utils.GetSetting(c, "store_name", "Cửa hàng")
		storeURL := utils.GetSetting(c, "store_url", "https://popshop.vn")

		// Link trỏ đến trang HTML reset password trên Next.js hoặc Server
		resetLink := fmt.Sprintf("%s/reset-password?token=%s", storeURL, resetToken)

		customerName := customer.FirstName + " " + customer.LastName
		if customerName == " " || customerName == "" {
			customerName = "Khách hàng"
		}

		reps := map[string]any{
			"CustomerName": customerName,
			"StoreName":    storeName,
			"ResetLink":    resetLink,
		}

		_ = tasks.EnqueueDynamicEmail(utils.AsynqClient, c.GetString("tenantId"), req.Email, "customer_reset_password", reps)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Nếu email tồn tại trong hệ thống, chúng tôi đã gửi hướng dẫn đặt lại mật khẩu.",
	})
}

func ResetPasswordHandler(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mật khẩu mới phải từ 6 ký tự trở lên"})
		return
	}

	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi kết nối DB"})
		return
	}

	// 1. Kiểm tra Token có hợp lệ & chưa hết hạn không
	var customerID uint64
	var expiresAt time.Time

	checkQuery := `
		SELECT id, reset_password_expires_at 
		FROM customers 
		WHERE reset_password_token = ? AND deleted_at IS NULL 
		LIMIT 1
	`
	err = tenantDB.QueryRow(checkQuery, req.Token).Scan(&customerID, &expiresAt)
	if err == sql.ErrNoRows || time.Now().After(expiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Liên kết đặt lại mật khẩu không hợp lệ hoặc đã hết hạn"})
		return
	}

	// 2. Hash mật khẩu mới
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể xử lý mật khẩu"})
		return
	}

	// 3. Cập nhật mật khẩu mới và XÓA Token
	updateQuery := `
		UPDATE customers 
		SET password = ?, 
		    reset_password_token = NULL, 
		    reset_password_expires_at = NULL, 
		    updated_at = NOW() 
		WHERE id = ?
	`
	_, err = tenantDB.Exec(updateQuery, string(hashedPassword), customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật mật khẩu thất bại"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Đổi mật khẩu thành công! Bạn có thể đăng nhập ngay bây giờ.",
	})
}

// 1. Render Trang Nhập Email
func RenderForgotPasswordPage(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Quên Mật Khẩu</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css" rel="stylesheet">
</head>
<body class="bg-slate-50 min-h-screen flex items-center justify-center p-4">
    <div class="max-w-md w-full bg-white rounded-2xl shadow-xl border border-slate-100 p-8">
        <div class="text-center mb-6">
            <div class="w-12 h-12 bg-blue-50 text-blue-600 rounded-full flex items-center justify-center mx-auto mb-3 text-xl">
                <i class="fa-solid fa-envelope"></i>
            </div>
            <h2 class="text-2xl font-bold text-slate-800">Quên Mật Khẩu?</h2>
            <p class="text-sm text-slate-500 mt-1">Nhập email tài khoản của bạn để nhận liên kết đặt lại mật khẩu.</p>
        </div>

        <div id="alertBox" class="hidden p-4 rounded-xl text-sm font-medium mb-6"></div>

        <form id="forgotForm" class="space-y-4">
            <div>
                <label class="block text-sm font-medium text-slate-700 mb-1">Email</label>
                <input type="email" id="email" required placeholder="nguyenvana@gmail.com"
                       class="w-full px-4 py-3 rounded-xl border border-slate-200 focus:ring-2 focus:ring-blue-500 outline-none text-sm">
            </div>

            <button type="submit" id="btnSubmit" 
                    class="w-full py-3.5 bg-blue-600 hover:bg-blue-700 text-white font-semibold rounded-xl shadow-lg shadow-blue-500/20 transition text-sm">
                Gửi Yêu Cầu
            </button>
        </form>
    </div>

    <script>
        const form = document.getElementById('forgotForm');
        const alertBox = document.getElementById('alertBox');
        const btnSubmit = document.getElementById('btnSubmit');

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            const email = document.getElementById('email').value;

            btnSubmit.disabled = true;
            btnSubmit.innerText = 'Đang gửi...';

            try {
                const res = await fetch('/api/v1/auth/forgot-password', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ email: email })
                });
                const data = await res.json();

                alertBox.classList.remove('hidden', 'bg-red-50', 'text-red-700', 'bg-green-50', 'text-green-700');
                if (res.ok) {
                    alertBox.classList.add('bg-green-50', 'text-green-700');
                    alertBox.innerText = data.message;
                    form.reset();
                } else {
                    alertBox.classList.add('bg-red-50', 'text-red-700');
                    alertBox.innerText = data.error || 'Có lỗi xảy ra!';
                }
            } catch (err) {
                alertBox.classList.remove('hidden');
                alertBox.classList.add('bg-red-50', 'text-red-700');
                alertBox.innerText = 'Không thể kết nối máy chủ!';
            } finally {
                btnSubmit.disabled = false;
                btnSubmit.innerText = 'Gửi Yêu Cầu';
            }
        });
    </script>
</body>
</html>`

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// 2. Render Trang Đặt Mật Khẩu Mới (Khi click link từ Email)
func RenderResetPasswordPage(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Đặt Lại Mật Khẩu</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css" rel="stylesheet">
</head>
<body class="bg-slate-50 min-h-screen flex items-center justify-center p-4">
    <div class="max-w-md w-full bg-white rounded-2xl shadow-xl border border-slate-100 p-8">
        <div class="text-center mb-6">
            <div class="w-12 h-12 bg-blue-50 text-blue-600 rounded-full flex items-center justify-center mx-auto mb-3 text-xl">
                <i class="fa-solid fa-lock"></i>
            </div>
            <h2 class="text-2xl font-bold text-slate-800">Đặt Lại Mật Khẩu</h2>
            <p class="text-sm text-slate-500 mt-1">Tạo mật khẩu mới cho tài khoản của bạn.</p>
        </div>

        <div id="alertBox" class="hidden p-4 rounded-xl text-sm font-medium mb-6"></div>

        <form id="resetForm" class="space-y-4">
            <div>
                <label class="block text-sm font-medium text-slate-700 mb-1">Mật khẩu mới</label>
                <input type="password" id="newPassword" required minlength="6" placeholder="Tối thiểu 6 ký tự"
                       class="w-full px-4 py-3 rounded-xl border border-slate-200 focus:ring-2 focus:ring-blue-500 outline-none text-sm">
            </div>

            <div>
                <label class="block text-sm font-medium text-slate-700 mb-1">Xác nhận mật khẩu</label>
                <input type="password" id="confirmPassword" required minlength="6" placeholder="Nhập lại mật khẩu mới"
                       class="w-full px-4 py-3 rounded-xl border border-slate-200 focus:ring-2 focus:ring-blue-500 outline-none text-sm">
            </div>

            <button type="submit" id="btnSubmit" 
                    class="w-full py-3.5 bg-blue-600 hover:bg-blue-700 text-white font-semibold rounded-xl shadow-lg shadow-blue-500/20 transition text-sm">
                Cập Nhật Mật Khẩu
            </button>
        </form>
    </div>

    <script>
        const urlParams = new URLSearchParams(window.location.search);
        const token = urlParams.get('token');

        const form = document.getElementById('resetForm');
        const alertBox = document.getElementById('alertBox');
        const btnSubmit = document.getElementById('btnSubmit');

        if (!token) {
            alertBox.classList.remove('hidden');
            alertBox.classList.add('bg-red-50', 'text-red-700');
            alertBox.innerText = 'Thiếu mã xác nhận! Vui lòng kiểm tra lại liên kết trong email.';
            form.style.display = 'none';
        }

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            const newPassword = document.getElementById('newPassword').value;
            const confirmPassword = document.getElementById('confirmPassword').value;

            if (newPassword !== confirmPassword) {
                alertBox.classList.remove('hidden');
                alertBox.classList.add('bg-red-50', 'text-red-700');
                alertBox.innerText = 'Mật khẩu xác nhận không trùng khớp!';
                return;
            }

            btnSubmit.disabled = true;
            btnSubmit.innerText = 'Đang xử lý...';

            try {
                const res = await fetch('/api/v1/auth/reset-password', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ token: token, new_password: newPassword })
                });
                const data = await res.json();

                alertBox.classList.remove('hidden', 'bg-red-50', 'text-red-700', 'bg-green-50', 'text-green-700');
                if (res.ok) {
                    alertBox.classList.add('bg-green-50', 'text-green-700');
                    alertBox.innerText = data.message || 'Đổi mật khẩu thành công!';
                    form.reset();
                } else {
                    alertBox.classList.add('bg-red-50', 'text-red-700');
                    alertBox.innerText = data.error || 'Mã xác thực đã hết hạn hoặc không hợp lệ!';
                }
            } catch (err) {
                alertBox.classList.remove('hidden');
                alertBox.classList.add('bg-red-50', 'text-red-700');
                alertBox.innerText = 'Lỗi kết nối máy chủ!';
            } finally {
                btnSubmit.disabled = false;
                btnSubmit.innerText = 'Cập Nhật Mật Khẩu';
            }
        });
    </script>
</body>
</html>`

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
