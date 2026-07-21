package generator

// Diff renders the configured structured concepts in memory and reports the
// diff against the active knowledge root without writing any file. It is the
// read-only core of drift detection: a non-empty Added/Changed/Removed set
// means the active bundle no longer matches its concept source.
func Diff(config *Config) (GenerateResult, error) {
	activeRoot, err := config.KnowledgeRoot()
	if err != nil {
		return GenerateResult{}, err
	}
	documents, err := renderedDocuments(config, activeRoot)
	if err != nil {
		return GenerateResult{}, err
	}
	active, err := hashActiveBundle(activeRoot, config.Security.Exclude)
	if err != nil {
		return GenerateResult{}, err
	}
	result := diffBundles(active, documents)
	result.Mode = "diff"
	return result, nil
}

// renderedDocuments builds the complete generated document set for the
// configuration: structured concepts when configured, otherwise the
// normalized passthrough of the active bundle, plus preserved logs.
func renderedDocuments(config *Config, activeRoot string) (map[string][]byte, error) {
	inputs, err := loadConcepts(config)
	if err != nil {
		return nil, err
	}
	var documents map[string][]byte
	if len(inputs) > 0 {
		concepts, err := resolveConcepts(config, inputs)
		if err != nil {
			return nil, err
		}
		documents, err = renderBundle(config, concepts)
		if err != nil {
			return nil, err
		}
	} else {
		documents, err = passthroughBundle(config, activeRoot)
		if err != nil {
			return nil, err
		}
	}
	if err := preserveLogs(activeRoot, config.Security.Exclude, documents); err != nil {
		return nil, err
	}
	return documents, nil
}
