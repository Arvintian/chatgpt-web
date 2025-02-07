package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	mmysql "github.com/go-sql-driver/mysql"
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
	Model    string `gorm:"column:model;not null;default:''"` // model_name,temperature,presence,frequency,max_tokens
	Isblock  int    `gorm:"column:is_block;not null;default:0"`
}

func (User) TableName() string {
	return "users"
}

func NewAccountService(dsn string, basicUsers, baiscPasswords string) (*AccountService, error) {
	accounts := map[string]string{}
	users := strings.Split(basicUsers, ",")
	passwords := strings.Split(baiscPasswords, ",")
	if len(users) != len(passwords) {
		return nil, errors.New("basic auth setting error")
	}
	for i := 0; i < len(users); i++ {
		if len(users[i]) > 0 {
			accounts[users[i]] = passwords[i]
		}
	}
	var db *gorm.DB
	_, err := mmysql.ParseDSN(dsn)
	if err != nil {
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	} else {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		return nil, err
	}
	as := &AccountService{
		db: db,
	}
	//init static user
	for user, passwd := range accounts {
		item, err := as.GetUser(user, passwd)
		if err != nil {
			klog.Infof("create user %s:%s", user, passwd)
			if err := as.CreateUser(user, passwd, -1); err != nil {
				return nil, err
			}
		} else {
			item.Balance, item.Isblock = -1, 0
			if err := db.Save(&item).Error; err != nil {
				return nil, err
			}
		}
	}
	return as, nil
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
	if payload.Action == "grant" {
		if err := ac.GrantUser(payload.Username, payload.Count); err != nil {
			ctx.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("%v", err),
			})
			return
		}
	}
	if payload.Action == "list" {
		users, err := ac.ListUser()
		if err != nil {
			ctx.JSON(http.StatusOK, gin.H{
				"status":  "Fail",
				"message": fmt.Sprintf("%v", err),
				"data":    nil,
			})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"status":  "Success",
			"message": "success",
			"data":    users,
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
}

func (ac *AccountService) ListUser() ([]User, error) {
	var users []User
	result := ac.db.Find(&users)
	return users, result.Error
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

func (ac *AccountService) GrantUser(username string, block int64) error {
	var user User
	result := ac.db.Where(&User{Username: username}).First(&user)
	if result.Error != nil {
		return result.Error
	}
	if block > 0 {
		user.Isblock = 1
	} else {
		user.Isblock = 0
	}
	result = ac.db.Save(&user)
	return result.Error
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

func (ac *AccountService) UpdateModel(username, model string) error {
	var user User
	result := ac.db.Where(&User{Username: username}).First(&user)
	if result.Error != nil {
		return result.Error
	}
	user.Model = model
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
	if user.Balance >= 0 && user.Usage >= user.Balance {
		return 2
	}
	return 0
}
