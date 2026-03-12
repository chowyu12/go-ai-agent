package gormstore

import (
	"context"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func (s *GormStore) CreateUser(ctx context.Context, u *model.User) error {
	return s.db.WithContext(ctx).Create(u).Error
}

func (s *GormStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (s *GormStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	if err := s.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (s *GormStore) ListUsers(ctx context.Context, q model.ListQuery) ([]*model.User, int64, error) {
	var items []*model.User
	var total int64

	db := s.db.WithContext(ctx).Model(&model.User{})
	if q.Keyword != "" {
		db = db.Where("username LIKE ?", "%"+q.Keyword+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset, limit := paginate(q)
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *GormStore) UpdateUser(ctx context.Context, id int64, req model.UpdateUserReq) error {
	updates := map[string]any{}
	if req.Password != nil {
		updates["password"] = *req.Password
	}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

func (s *GormStore) DeleteUser(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

func (s *GormStore) HasAdmin(ctx context.Context) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
