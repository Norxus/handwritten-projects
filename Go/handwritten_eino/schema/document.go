package schema

const (
	docMetaDataKeySubIndexes   = "_sub_indexes"
	docMetaDataKeyScore        = "_score"
	docMetaDataKeyExtraInfo    = "_extra_info"
	docMetaDataKeyDSL          = "_dsl_info"
	docMetaDataKeyDenseVector  = "_dense_vector"
	docMetaDataKeySparseVector = "_sparse_vector"
)

type Document struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	MetaData map[string]any `json:"meta_data"`
}

func (d *Document) String() string {
	return d.Content
}

func (d *Document) WithSubIndexes(indexes []string) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeySubIndexes] = indexes
	return d
}

func (d *Document) SubIndexes() []string {
	if d.MetaData == nil {
		return nil
	}

	indexes, ok := d.MetaData[docMetaDataKeySubIndexes].([]string)
	if !ok {
		return nil
	}
	return indexes
}

func (d *Document) WithScore(score float64) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeyScore] = score
	return d
}

func (d *Document) Score() float64 {
	if d.MetaData == nil {
		return 0
	}

	score, ok := d.MetaData[docMetaDataKeyScore].(float64)
	if ok {
		return score
	}

	return 0
}

func (d *Document) WithExtraInfo(extraInfo string) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeyExtraInfo] = extraInfo
	return d
}

func (d *Document) ExtraInfo() string {
	if d.MetaData == nil {
		return ""
	}

	extraInfo, ok := d.MetaData[docMetaDataKeyExtraInfo].(string)
	if ok {
		return extraInfo
	}

	return ""
}

func (d *Document) WithDSLInfo(dslInfo map[string]any) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeyDSL] = dslInfo
	return d
}

func (d *Document) DSLInfo() map[string]any {
	if d.MetaData == nil {
		return nil
	}

	dslInfo, ok := d.MetaData[docMetaDataKeyDSL].(map[string]any)
	if ok {
		return dslInfo
	}

	return nil
}

func (d *Document) WithDenseVector(vector []float64) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeyDenseVector] = vector
	return d
}

func (d *Document) DenseVector() []float64 {
	if d.MetaData == nil {
		return nil
	}
	vector, ok := d.MetaData[docMetaDataKeyDenseVector].([]float64)
	if ok {
		return vector
	}
	return nil
}

func (d *Document) WithSparseVector(sparse map[int]float64) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeySparseVector] = sparse

	return d
}

// SparseVector returns the sparse vector of the document, key indices -> value vector.
// can use doc.WithSparseVector() to set the sparse vector.
func (d *Document) SparseVector() map[int]float64 {
	if d.MetaData == nil {
		return nil
	}

	sparse, ok := d.MetaData[docMetaDataKeySparseVector].(map[int]float64)
	if ok {
		return sparse
	}

	return nil
}
