package git

// Helper function to convert map[string]string to map[string]GitObject for tests
func toGitObjects(m map[string]string) map[string]GitObject {
	result := make(map[string]GitObject)
	for k, v := range m {
		result[k] = GitObject{
			Content: []byte(v),
		}
	}
	return result
}
