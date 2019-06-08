// Code generated by MockGen. DO NOT EDIT.
// Source: utxoviewpoint.go

// Package mock_blockchain is a generated GoMock package.
package mock_blockchain

import (
	utxo "github.com/btcsuite/btcd/utxo"
	wire "github.com/btcsuite/btcd/wire"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockUtxoViewpointInterface is a mock of UtxoViewpointInterface interface
type MockUtxoViewpointInterface struct {
	ctrl     *gomock.Controller
	recorder *MockUtxoViewpointInterfaceMockRecorder
}

// MockUtxoViewpointInterfaceMockRecorder is the mock recorder for MockUtxoViewpointInterface
type MockUtxoViewpointInterfaceMockRecorder struct {
	mock *MockUtxoViewpointInterface
}

// NewMockUtxoViewpointInterface creates a new mock instance
func NewMockUtxoViewpointInterface(ctrl *gomock.Controller) *MockUtxoViewpointInterface {
	mock := &MockUtxoViewpointInterface{ctrl: ctrl}
	mock.recorder = &MockUtxoViewpointInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockUtxoViewpointInterface) EXPECT() *MockUtxoViewpointInterfaceMockRecorder {
	return m.recorder
}

// LookupEntry mocks base method
func (m *MockUtxoViewpointInterface) LookupEntry(arg0 wire.OutPoint) *utxo.UtxoEntry {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LookupEntry", arg0)
	ret0, _ := ret[0].(*utxo.UtxoEntry)
	return ret0
}

// LookupEntry indicates an expected call of LookupEntry
func (mr *MockUtxoViewpointInterfaceMockRecorder) LookupEntry(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LookupEntry", reflect.TypeOf((*MockUtxoViewpointInterface)(nil).LookupEntry), arg0)
}