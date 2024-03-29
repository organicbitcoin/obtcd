// Code generated by MockGen. DO NOT EDIT.
// Source: chain.go

// Package mock_blockchain is a generated GoMock package.
package mock_blockchain

import (
	utxo "github.com/organicbitcoin/obtcd/utxo"
	wire "github.com/organicbitcoin/obtcd/wire"
	btcutil "github.com/organicbitcoin/btcutil"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockInterface is a mock of Interface interface
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
}

// MockInterfaceMockRecorder is the mock recorder for MockInterface
type MockInterfaceMockRecorder struct {
	mock *MockInterface
}

// NewMockInterface creates a new mock instance
func NewMockInterface(ctrl *gomock.Controller) *MockInterface {
	mock := &MockInterface{ctrl: ctrl}
	mock.recorder = &MockInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockInterface) EXPECT() *MockInterfaceMockRecorder {
	return m.recorder
}

// BlockByHeight mocks base method
func (m *MockInterface) BlockByHeight(height int32) (*btcutil.Block, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockByHeight", height)
	ret0, _ := ret[0].(*btcutil.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockByHeight indicates an expected call of BlockByHeight
func (mr *MockInterfaceMockRecorder) BlockByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockByHeight", reflect.TypeOf((*MockInterface)(nil).BlockByHeight), height)
}

// FetchUtxosByHeight mocks base method
func (m *MockInterface) FetchUtxosByHeight(height int32) (map[wire.OutPoint]*utxo.UtxoEntry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchUtxosByHeight", height)
	ret0, _ := ret[0].(map[wire.OutPoint]*utxo.UtxoEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchUtxosByHeight indicates an expected call of FetchUtxosByHeight
func (mr *MockInterfaceMockRecorder) FetchUtxosByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchUtxosByHeight", reflect.TypeOf((*MockInterface)(nil).FetchUtxosByHeight), height)
}
