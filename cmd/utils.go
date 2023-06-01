package main

import (
	"errors"
	"regexp"
)

// ExtractAccountAndPassword 从字符串中提取账户密码并校验
func ExtractAccountAndPassword(str string) (account, password string, err error) {
	pattern := `^/user ([a-zA-Z][a-zA-Z0-9]{3,11}):([a-zA-Z0-9]{6,12})$`
	re := regexp.MustCompile(pattern)
	result := re.FindStringSubmatch(str)

	if len(result) == 3 { // 匹配成功，result[0]为整个字符串，result[1]为账户，result[2]为密码
		account = result[1]
		password = result[2]

		// 校验账户和密码
		if !checkAccount(account) {
			err = errors.New("账户不合法")
		} else if !checkPassword(password) {
			err = errors.New("密码不合法")
		}
	} else {
		err = errors.New("账户密码格式不正确")
	}

	return
}

// 校验账户
func checkAccount(account string) bool {
	pattern := `^[a-zA-Z][a-zA-Z0-9]{3,11}$`
	re := regexp.MustCompile(pattern)
	return re.MatchString(account)
}

// 校验密码
func checkPassword(password string) bool {
	pattern := `^[a-zA-Z0-9]{6,12}$`
	re := regexp.MustCompile(pattern)
	return re.MatchString(password)
}
