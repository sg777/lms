#!/bin/bash

echo "🧪 Running all signature verification tests..."
echo ""

echo "=========================================="
echo "Testing HSM Server Signature Creation"
echo "=========================================="
go test -v ./hsm_server -run TestCommitIndexToRaft_SignatureCreation
go test -v ./hsm_server -run TestSignature_DataFormat
go test -v ./hsm_server -run TestSignature_WithCorrectFormat
echo ""

echo "=========================================="
echo "Testing FSM Signature Verification"
echo "=========================================="
go test -v ./fsm -run TestKeyIndexFSM_VerifySignature
go test -v ./fsm -run TestKeyIndexFSM_Apply_ValidSignature
go test -v ./fsm -run TestKeyIndexFSM_Apply_InvalidSignature
go test -v ./fsm -run TestKeyIndexFSM_DataFormatConsistency
go test -v ./fsm -run TestKeyIndexFSM_EndToEnd
echo ""

echo "=========================================="
echo "Running ALL tests in hsm_server and fsm packages"
echo "=========================================="
go test -v ./hsm_server ./fsm
echo ""

echo "✅ All tests completed!"

