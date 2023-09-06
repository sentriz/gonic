package share

import (
	"errors"
	"fmt"

	"go.senan.xyz/gonic/db"
)

var ErrInvalidPathFormat = errors.New("invalid path format")
var ErrInvalidBasePath = errors.New("invalid base path")
var ErrNoUserPrefix = errors.New("no user prefix")

type Share struct {
	db    *db.DB
	uiURL string
}

func New(db *db.DB, uiURL string) *Share {
	if string(uiURL[len(uiURL)-1]) == "/" {
		uiURL = uiURL[:len(uiURL)-1]
	}

	return &Share{
		db:    db,
		uiURL: uiURL,
	}
}

func (s *Share) GetURL(title string) string {

	return fmt.Sprintf("%s/%s", s.uiURL, title)
}

func (s *Share) GetShares(userID int, title string) ([]*db.Share, error) {
	var err error
	shares := []*db.Share{}
	switch {
	case userID == 0:
		err = s.db.Where("title=?", title).Find(&shares).Error
	case title != "":
		err = s.db.Where("user_id=? AND title=?", userID, title).Find(&shares).Error
	default:
		err = s.db.Where("user_id=?", userID).Find(&shares).Error
	}

	if err != nil {
		return nil, fmt.Errorf("finding shares: %w", err)
	}

	for _, so := range shares {
		se, err := s.GetShareEntry(so.ID)
		if err != nil {
			return nil, fmt.Errorf("finding share tracks: %w", err)
		}
		so.Entries = se
	}

	return shares, nil
}

func (s *Share) GetShareEntry(shareID int) ([]string, error) {
	entries := []*db.ShareEntry{}
	err := s.db.Where("share_id=?", shareID).Find(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("find episodes by podcast id: %w", err)
	}

	res := []string{}
	for _, e := range entries {
		res = append(res, e.Entry)
	}

	return res, nil
}

func (s *Share) Save(share *db.Share) error {
	if err := s.db.Save(&share).Error; err != nil {
		return err
	}

	return s.SaveEntries(share)
}

func (s *Share) SaveEntries(share *db.Share) error {
	shareEntries := []*db.ShareEntry{}
	err := s.db.Where("share_id=?", share.ID).Find(&shareEntries).Error
	if err != nil {
		return err
	}

	// add new
	for _, cEntry := range share.Entries {
		founded := false
		for _, entry := range shareEntries {
			if entry.Entry == cEntry {
				founded = true
				break
			}
		}
		if !founded {
			if err := s.db.Save(&db.ShareEntry{ShareID: share.ID, Entry: cEntry}).Error; err != nil {
				return err
			}
		}
	}

	// delete old
	for i, entry := range shareEntries {
		founded := false
		for _, cEntry := range share.Entries {
			if entry.Entry == cEntry {
				founded = true
				break
			}
		}
		if !founded {
			if err := s.db.Delete(&shareEntries[i]).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Share) Delete(userID int, title string) error {
	err := s.db.Where("user_id=? AND title=?", userID, title).Delete(db.Share{}).Error
	if err != nil {
		return fmt.Errorf("delete share row: %w", err)
	}
	return nil
}
