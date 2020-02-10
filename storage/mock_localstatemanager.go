package storage

// MockLocalStateManager is a mock use for test purpose
type MockLocalStateManager struct {
}

func (s *MockLocalStateManager) SaveLocalState(state KeygenLocalStateItem) error {
	return nil
}
func (s *MockLocalStateManager) GetLocalState(pubKey string) (KeygenLocalStateItem, error) {
	return KeygenLocalStateItem{}, nil
}
