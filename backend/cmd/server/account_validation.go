package main

import (
	"strings"
	"unicode/utf8"
)

func validateAccountUsername(value string) (string, string) {
	username := strings.TrimSpace(value)
	if utf8.RuneCountInString(username) < 3 {
		return "", "账号至少需要 3 个字符"
	}
	if len(username) > 80 {
		return "", "账号不能超过 80 个字节"
	}
	return username, ""
}

func validateAccountPassword(password string) string {
	if utf8.RuneCountInString(password) < 6 {
		return "密码至少需要 6 个字符"
	}
	if len(password) > 72 {
		return "密码不能超过 72 个字节"
	}
	return ""
}

func isUniqueUsernameError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key")
}
