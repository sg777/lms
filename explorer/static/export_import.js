// Export/Import/Delete key functionality

async function exportKey(keyId) {
    if (!authToken) {
        alert('Please login first');
        return;
    }

    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/export`, {
            method: 'POST',
            body: JSON.stringify({
                key_id: keyId
            })
        });

        if (!response.ok) {
            if (response.status === 401) {
                handleLogout();
                return;
            }
            const data = await response.json();
            throw new Error(data.error || 'Failed to export key');
        }

        const data = await response.json();
        
        if (data.success) {
            // Create a downloadable JSON file (keys are already base64 strings)
            const exportData = {
                key_id: data.key_id,
                private_key: data.private_key, // Base64 string
                public_key: data.public_key,   // Base64 string
                index: data.index,
                params: data.params,
                levels: data.levels,
                lm_type: data.lm_type,
                ots_type: data.ots_type,
                created: data.created
            };
            
            const jsonStr = JSON.stringify(exportData, null, 2);
            const blob = new Blob([jsonStr], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `lms_key_${keyId}_export.json`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            
            alert(`Key exported successfully!\nFile: lms_key_${keyId}_export.json\n\n⚠️ Keep this file secure - it contains your private key!`);
        } else {
            throw new Error(data.error || 'Export failed');
        }
    } catch (error) {
        alert('Error exporting key: ' + error.message);
    }
}

async function deleteKey(keyId) {
    if (!authToken) {
        alert('Please login first');
        return;
    }

    if (!confirm(`Are you sure you want to delete key "${keyId}"?\n\n⚠️ This action cannot be undone!\n\nNote: Raft chain entries for this key will remain, but the key will be removed from your HSM.`)) {
        return;
    }

    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/delete`, {
            method: 'POST',
            body: JSON.stringify({
                key_id: keyId
            })
        });

        if (!response.ok) {
            if (response.status === 401) {
                handleLogout();
                return;
            }
            const data = await response.json();
            throw new Error(data.error || 'Failed to delete key');
        }

        const data = await response.json();
        
        if (data.success) {
            alert(`Key "${keyId}" deleted successfully`);
            // Reload keys list
            await loadMyKeys();
        } else {
            throw new Error(data.error || 'Delete failed');
        }
    } catch (error) {
        alert('Error deleting key: ' + error.message);
    }
}

async function handleImportKey(e) {
    e.preventDefault();
    
    if (!authToken) {
        alert('Please login first');
        return;
    }

    const jsonData = document.getElementById('importKeyData').value.trim();
    const newKeyId = document.getElementById('importKeyId').value.trim();
    const errorDiv = document.getElementById('importKeyError');
    
    errorDiv.style.display = 'none';
    
    if (!jsonData) {
        errorDiv.textContent = 'Please paste the exported key JSON';
        errorDiv.style.display = 'block';
        return;
    }

    try {
        // Parse JSON to validate
        const importedData = JSON.parse(jsonData);
        
        // Keys should already be base64 strings in the exported JSON
        const importRequest = {
            key_id: newKeyId || '', // Empty = auto-generate
            private_key: importedData.private_key || '',
            public_key: importedData.public_key || '',
            index: importedData.index !== undefined ? importedData.index : 0,
            params: importedData.params || '',
            levels: importedData.levels || 1,
            lm_type: importedData.lm_type || [],
            ots_type: importedData.ots_type || []
        };

        // Validate keys are base64 strings
        if (typeof importRequest.private_key !== 'string' || importRequest.private_key === '') {
            throw new Error('Invalid private_key format: expected base64 string');
        }
        if (typeof importRequest.public_key !== 'string' || importRequest.public_key === '') {
            throw new Error('Invalid public_key format: expected base64 string');
        }

        const response = await authenticatedFetch(`${API_BASE}/api/my/import`, {
            method: 'POST',
            body: JSON.stringify({
                key_id: importRequest.key_id,
                private_key: importRequest.private_key, // Base64 string
                public_key: importRequest.public_key,   // Base64 string
                index: importRequest.index,
                params: importRequest.params,
                levels: importRequest.levels,
                lm_type: importRequest.lm_type,
                ots_type: importRequest.ots_type
            })
        });

        if (!response.ok) {
            if (response.status === 401) {
                handleLogout();
                return;
            }
            const data = await response.json();
            throw new Error(data.error || 'Failed to import key');
        }

        const data = await response.json();
        
        if (data.success) {
            closeModal('importKeyModal');
            document.getElementById('importKeyForm').reset();
            alert(`Key imported successfully!\nNew Key ID: ${data.key_id}`);
            // Reload keys list
            await loadMyKeys();
        } else {
            throw new Error(data.error || 'Import failed');
        }
    } catch (error) {
        if (error instanceof SyntaxError) {
            errorDiv.textContent = 'Invalid JSON format: ' + error.message;
        } else {
            errorDiv.textContent = error.message;
        }
        errorDiv.style.display = 'block';
    }
}

