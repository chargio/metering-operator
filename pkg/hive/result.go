package hive

// CreateReportTable creates a new table backed by the given bucket/prefix with
// the specified columns
func CreateReportTable(queryer Queryer, tableName, bucket, prefix string, columns []Column) error {
	location, err := s3Location(bucket, prefix)
	if err != nil {
		return err
	}

	query := dropTable(tableName, true, true)
	err = queryer.Query(query)
	if err != nil {
		return err
	}

	query = createTable(tableName, location, "", "", nil, columns, nil, false, false)
	return queryer.Query(query)
}

func CreateLocalReportTable(queryer Queryer, tableName string, columns []Column) error {
	query := dropTable(tableName, true, true)
	err := queryer.Query(query)
	if err != nil {
		return err
	}

	query = createTable(tableName, "", "", "", nil, columns, nil, false, true)
	return queryer.Query(query)
}
