package store

import "jenkinsAgent/internal/model"

type UserStore struct{}

func NewUserStore() *UserStore {
	return &UserStore{}
}

func (s *UserStore) Create(u *model.User) error {
	return DB.Create(u).Error
}

func (s *UserStore) GetByUsername(username string) (*model.User, error) {
	var u model.User
	err := DB.Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) GetByID(id uint) (*model.User, error) {
	var u model.User
	err := DB.First(&u, id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) Count() (int64, error) {
	var count int64
	err := DB.Model(&model.User{}).Count(&count).Error
	return count, err
}
