package write

// StationModel is the persistence mapping for table stations.
// Schema is owned by SQL migrations — do not encode DDL in tags.
// The ingester only reads content_hash. The worker owns writes.
type StationModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	ContentHash string `gorm:"column:content_hash"`
}

func (StationModel) TableName() string {
	return "stations"
}
