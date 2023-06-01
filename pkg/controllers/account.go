package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

type AccountService struct {
	db *gorm.DB
}

type User struct {
	ID       int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Username string `gorm:"column:username;not null;unique;index"`
	Password string `gorm:"column:password;not null;index"`
	Balance  int64  `gorm:"column:balance;not null;default:0"`
	Usage    int64  `gorm:"column:usage;not null;default:0"`
	Isblock  int    `gorm:"column:is_block;not null;default:0"`
}

func (User) TableName() string {
	return "users"
}

func NewAccountService(dsn string) (*AccountService, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&User{})
	return &AccountService{
		db: db,
	}, nil
}

type AccountPayload struct {
	Action   string `json:"action"`
	Count    int64  `json:"count"`
	Username string `json:"i_username"`
	Password string `json:"i_password"`
}

func (ac *AccountService) AccountProcess(ctx *gin.Context) {
	payload := AccountPayload{}
	if err := ctx.BindJSON(&payload); err != nil {
		klog.Error(err)
		ctx.JSON(200, gin.H{
			"status":  "Fail",
			"message": fmt.Sprintf("%v", err),
			"data":    nil,
		})
		return
	}
	if payload.Action == "recharge" {
		if err := ac.IncBalance(payload.Username, payload.Count); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"message": fmt.Sprintf("%v", err),
			})
			return
		}
	}
	if payload.Action == "check" {
		if _, err := ac.CheckUser(payload.Username); err != nil {
			ctx.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("%v", err),
			})
			return
		}
	}
	if payload.Action == "register" {
		if err := ac.CreateUser(payload.Username, payload.Password, payload.Count); err != nil {
			ctx.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("%v", err),
			})
			return
		}
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
}

func (ac *AccountService) CreateUser(name, password string, cnt int64) error {
	var exist User
	result := ac.db.Where(&User{Username: name}).First(&exist)
	if result.Error == nil {
		return errors.New("账户名存在")
	}
	var user User
	user.Username = name
	user.Password = password
	user.Balance = cnt
	result = ac.db.Create(&user)
	if result.Error != nil && strings.Contains(result.Error.Error(), "Duplicate") {
		return errors.New("账户名存在")
	}
	return result.Error
}

func (ac *AccountService) UpdateUser(oldName, oldPassword, name, password string) error {
	var exist User
	result := ac.db.Where(&User{Username: name}).First(&exist)
	if result.Error == nil && exist.Password != oldPassword {
		return errors.New("账户名存在")
	}
	var user User
	result = ac.db.Where(&User{Username: oldName, Password: oldPassword}).First(&user)
	if result.Error != nil {
		return result.Error
	}
	user.Username = name
	user.Password = password
	result = ac.db.Save(&user)
	if result.Error != nil && strings.Contains(result.Error.Error(), "Duplicate") {
		return errors.New("账户名存在")
	}
	return result.Error
}

func (ac *AccountService) GetUser(username, password string) (User, error) {
	var user User
	result := ac.db.Where(&User{Username: username, Password: password}).First(&user)
	if result.Error != nil {
		return user, result.Error
	}
	return user, nil
}

func (ac *AccountService) CheckUser(username string) (User, error) {
	var user User
	result := ac.db.Where(&User{Username: username}).First(&user)
	if result.Error != nil {
		return user, result.Error
	}
	return user, nil
}

func (ac *AccountService) IncBalance(username string, cnt int64) error {
	var user User
	result := ac.db.Where(&User{Username: username}).First(&user)
	if result.Error != nil {
		return result.Error
	}
	user.Balance = user.Balance + int64(cnt)
	result = ac.db.Save(&user)
	return result.Error
}

func (ac *AccountService) IncUsage(username string, cnt int64) error {
	var user User
	result := ac.db.Where(&User{Username: username}).First(&user)
	if result.Error != nil {
		return result.Error
	}
	user.Usage = user.Usage + int64(cnt)
	result = ac.db.Save(&user)
	return result.Error
}

func (ac *AccountService) AuthenticateUser(username, password string) int {
	var user User
	result := ac.db.Where(&User{Username: username, Password: password}).First(&user)
	if result.Error != nil {
		return 1
	}
	if user.Isblock > 0 {
		return 1
	}
	if user.Usage >= user.Balance {
		return 2
	}
	return 0
}
