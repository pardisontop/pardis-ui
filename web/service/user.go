package service

import (
	"errors"

	"github.com/alireza0/pardis-ui/database"
	"github.com/alireza0/pardis-ui/database/model"
	"github.com/alireza0/pardis-ui/logger"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct{}

func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) CheckUser(username string, password string) *model.User {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		// Transparent migration: support legacy plain-text passwords
		if user.Password != password {
			return nil
		}
		// Upgrade plain-text password to bcrypt hash on successful login
		if hashed, hashErr := hashPassword(password); hashErr == nil {
			db.Model(user).Update("password", hashed)
			user.Password = hashed
		}
	}
	return user
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"username": username, "password": hashed}).
		Error
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return errors.New("username can not be empty")
	} else if password == "" {
		return errors.New("password can not be empty")
	}
	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	db := database.GetDB()
	user := &model.User{}
	err = db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashed
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashed
	return db.Save(user).Error
}
