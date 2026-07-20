package handler

import (
	"html/template"
	"log"
	"net/http"

	"jenkinsAgent/config"
	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	tmpl      map[string]*template.Template
	userStore *store.UserStore
}

func NewAuthHandler(tmpl map[string]*template.Template) *AuthHandler {
	return &AuthHandler{
		tmpl:      tmpl,
		userStore: store.NewUserStore(),
	}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	_ = h.tmpl["login"].ExecuteTemplate(w, "login.html", map[string]interface{}{
		"Error": "",
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.userStore.GetByUsername(username)
	if err != nil {
		_ = h.tmpl["login"].ExecuteTemplate(w, "login.html", map[string]interface{}{
			"Error": "用户名或密码错误",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		_ = h.tmpl["login"].ExecuteTemplate(w, "login.html", map[string]interface{}{
			"Error": "用户名或密码错误",
		})
		return
	}

	middleware.SetSession(w, username, config.Global.Server.SecretKey)
	log.Printf("[auth] user %s logged in", username)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	middleware.ClearSession(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// InitAdmin creates the admin user if no users exist.
func InitAdmin() error {
	userStore := store.NewUserStore()
	count, err := userStore.Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(config.Global.Auth.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &model.User{
		Username:     config.Global.Auth.AdminUser,
		PasswordHash: string(hash),
		Role:         model.RoleAdmin,
	}
	if err := userStore.Create(admin); err != nil {
		return err
	}
	log.Printf("[auth] admin user '%s' created", admin.Username)
	return nil
}
