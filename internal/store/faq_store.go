package store

import "jenkinsAgent/internal/model"

type FAQStore struct{}

func NewFAQStore() *FAQStore { return &FAQStore{} }

func (s *FAQStore) Create(faq *model.FAQ) error {
	return DB.Create(faq).Error
}

func (s *FAQStore) ListAll() ([]model.FAQ, error) {
	var faqs []model.FAQ
	err := DB.Order("created_at desc").Find(&faqs).Error
	return faqs, err
}

func (s *FAQStore) GetByID(id uint) (*model.FAQ, error) {
	var faq model.FAQ
	err := DB.First(&faq, id).Error
	return &faq, err
}

func (s *FAQStore) Update(faq *model.FAQ) error {
	return DB.Save(faq).Error
}

func (s *FAQStore) Delete(id uint) error {
	return DB.Delete(&model.FAQ{}, id).Error
}
