package model

const (
	AiUnionMediaKindVideo     = "video"
	AiUnionMediaKindLastFrame = "last_frame"
	AiUnionMediaKindAsset     = "asset"

	AiUnionMediaStatusPending     = "pending"
	AiUnionMediaStatusDownloading = "downloading"
	AiUnionMediaStatusReady       = "ready"
	AiUnionMediaStatusFailed      = "failed"
)

type AiUnionMedia struct {
	ID              int64  `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	UserId          int    `json:"user_id" gorm:"index"`
	TaskID          string `json:"task_id" gorm:"type:varchar(191);index"`
	Kind            string `json:"kind" gorm:"type:varchar(32);index"`
	Status          string `json:"status" gorm:"type:varchar(32);index"`
	StoragePath     string `json:"-" gorm:"type:text"`
	FileName        string `json:"file_name" gorm:"type:varchar(255)"`
	MimeType        string `json:"mime_type" gorm:"type:varchar(128)"`
	SizeBytes       int64  `json:"size_bytes"`
	Sha256          string `json:"sha256" gorm:"type:varchar(64)"`
	SourceURL       string `json:"-" gorm:"type:text"`
	SourceExpiresAt int64  `json:"source_expires_at" gorm:"index"`
	DownloadedAt    int64  `json:"downloaded_at"`
	Error           string `json:"error" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at"`
}

func GetAiUnionMediaByID(id int64) (*AiUnionMedia, bool, error) {
	if id == 0 {
		return nil, false, nil
	}
	var media AiUnionMedia
	err := DB.First(&media, id).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return &media, exist, nil
}

func GetAiUnionMediaByUser(id int64, userID int) (*AiUnionMedia, bool, error) {
	if id == 0 || userID == 0 {
		return nil, false, nil
	}
	var media AiUnionMedia
	err := DB.Where("id = ? AND user_id = ?", id, userID).First(&media).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return &media, exist, nil
}

func GetAiUnionMediaByTask(userID int, taskID string) ([]*AiUnionMedia, error) {
	if userID == 0 || taskID == "" {
		return nil, nil
	}
	var media []*AiUnionMedia
	err := DB.Where("user_id = ? AND task_id = ?", userID, taskID).
		Order("id asc").
		Find(&media).Error
	return media, err
}
