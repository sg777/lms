// Signature verification functionality

// Handle file upload for signature
document.addEventListener('DOMContentLoaded', function() {
    const fileInput = document.getElementById('verifySignatureFile');
    const textarea = document.getElementById('verifySignatureInput');
    
    if (fileInput && textarea) {
        fileInput.addEventListener('change', function(e) {
            const file = e.target.files[0];
            if (file) {
                const reader = new FileReader();
                reader.onload = function(event) {
                    try {
                        const content = event.target.result;
                        // Validate it's valid JSON and format it
                        const parsed = JSON.parse(content);
                        textarea.value = JSON.stringify(parsed, null, 2);
                    } catch (error) {
                        showCopyableError('Error reading file: ' + error.message);
                    }
                };
                reader.readAsText(file);
            }
        });
    }
});

async function handleVerifySignature(e) {
    e.preventDefault();
    
    if (!authToken) {
        alert('Please login first');
        return;
    }

    const keyID = document.getElementById('verifyKeySelect').value; // Optional
    const message = document.getElementById('verifyMessageInput').value.trim();
    const signatureText = document.getElementById('verifySignatureInput').value.trim();
    const resultDiv = document.getElementById('verifyResult');
    
    if (!message) {
        resultDiv.innerHTML = '<div class="error-message">Please enter the original message</div>';
        resultDiv.style.display = 'block';
        return;
    }
    
    if (!signatureText) {
        resultDiv.innerHTML = '<div class="error-message">Please paste or upload the signature JSON</div>';
        resultDiv.style.display = 'block';
        return;
    }
    
    // Parse signature JSON
    let signatureObj;
    try {
        signatureObj = JSON.parse(signatureText);
    } catch (error) {
        resultDiv.innerHTML = `<div class="error-message">Invalid signature JSON: ${escapeHtml(error.message)}</div>`;
        resultDiv.style.display = 'block';
        return;
    }
    
    // Validate signature structure
    if (!signatureObj.pubkey || !signatureObj.signature || signatureObj.index === undefined) {
        resultDiv.innerHTML = '<div class="error-message">Invalid signature format. Expected: { "pubkey": "...", "index": 0, "signature": "..." }</div>';
        resultDiv.style.display = 'block';
        return;
    }
    
    const btn = document.getElementById('verifyBtn');
    const originalText = btn.textContent;
    btn.textContent = 'Verifying...';
    btn.disabled = true;
    resultDiv.style.display = 'none';
    
    try {
        const requestBody = {
            key_id: keyID || '', // Empty if not selected
            signature: signatureObj,
            message: message
        };
        
        const response = await authenticatedFetch(`${API_BASE}/api/my/verify`, {
            method: 'POST',
            body: JSON.stringify(requestBody)
        });
        
        if (!response.ok) {
            if (response.status === 401) {
                handleLogout();
                return;
            }
            const data = await response.json();
            throw new Error(data.error || 'Failed to verify signature');
        }
        
        const data = await response.json();
        
        if (data.success) {
            const isValid = data.valid === true;
            const badgeClass = isValid ? 'success-message' : 'error-message';
            const badgeIcon = isValid ? '✅' : '❌';
            const badgeText = isValid ? 'VALID' : 'INVALID';
            
            resultDiv.innerHTML = `
                <div class="${badgeClass}">
                    <strong>${badgeIcon} Signature Verification: ${badgeText}</strong><br><br>
                    <strong>Index:</strong> ${signatureObj.index}<br>
                    <strong>Public Key (first 64 chars):</strong> ${escapeHtml(signatureObj.pubkey.substring(0, 64))}...<br>
                    <strong>Message:</strong> ${escapeHtml(message.substring(0, 100))}${message.length > 100 ? '...' : ''}<br><br>
                    ${isValid 
                        ? '<div style="color: #10b981; font-weight: bold;">The signature is cryptographically valid and matches the message and public key.</div>'
                        : '<div style="color: #ef4444; font-weight: bold;">The signature verification failed. The signature may be corrupted, the message may have been modified, or the wrong public key was used.</div>'
                    }
                </div>
            `;
            resultDiv.style.display = 'block';
        } else {
            throw new Error(data.error || 'Verification failed');
        }
    } catch (error) {
        resultDiv.innerHTML = `<div class="error-message">Error verifying signature: ${escapeHtml(error.message)}</div>`;
        resultDiv.style.display = 'block';
    } finally {
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

// Escape HTML helper (if not already defined)
function escapeHtml(text) {
    if (typeof text !== 'string') return text;
    const map = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#039;'
    };
    return text.replace(/[&<>"']/g, m => map[m]);
}

