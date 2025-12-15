// Wallet management for CHIPS keys

async function loadWallets(silent = false) {
    const walletList = document.getElementById('walletList');
    if (!walletList) return;

    if (!silent) {
        walletList.innerHTML = '<div class="loading">Loading wallets...</div>';
    }

    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/wallet/list`);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const data = await response.json();
        if (!data.success) {
            throw new Error(data.error || 'Failed to load wallets');
        }

        displayWallets(data.wallets || [], walletList);
    } catch (error) {
        walletList.innerHTML = `<div class="error">Error loading wallets: ${escapeHtml(error.message)}</div>`;
        console.error('Wallet load error:', error);
    }
}

function displayWallets(wallets, container) {
    if (!wallets || wallets.length === 0) {
        container.innerHTML = `
            <div class="info">
                <p>No CHIPS wallets found. Create your first wallet to start using blockchain features.</p>
            </div>
        `;
        return;
    }

    let html = `
        <table class="data-table">
            <thead>
                <tr>
                    <th>Address (Public Key)</th>
                    <th>Balance (CHIPS)</th>
                    <th>Created</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
    `;

    wallets.forEach((wallet) => {
        const balance = wallet.balance || 0;
        const balanceDisplay = balance.toFixed(8);
        const balanceClass = balance > 0 ? 'positive' : 'zero';
        
        html += `
            <tr>
                <td class="hash-cell" title="${escapeHtml(wallet.address)}">
                    <code>${escapeHtml(wallet.address)}</code>
                </td>
                <td class="balance ${balanceClass}">
                    <strong>${balanceDisplay}</strong> CHIPS
                </td>
                <td>${formatDate(wallet.created_at)}</td>
                <td>
                    <button class="refresh-btn" onclick="refreshWalletBalance('${escapeHtml(wallet.address)}')" style="font-size: 12px; padding: 4px 8px;">
                        🔄 Refresh
                    </button>
                </td>
            </tr>
        `;
    });

    html += '</tbody></table>';
    container.innerHTML = html;
}

async function createWallet() {
    const createBtn = document.getElementById('createWalletBtn');
    if (createBtn) {
        createBtn.disabled = true;
        createBtn.textContent = 'Creating...';
    }

    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/wallet/create`, {
            method: 'POST',
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || `HTTP ${response.status}`);
        }

        const data = await response.json();
        if (!data.success) {
            throw new Error(data.error || 'Failed to create wallet');
        }

        // Show success message
        alert(`Wallet created successfully!\n\nAddress: ${data.wallet.address}\nBalance: ${data.wallet.balance || 0} CHIPS`);

        // Reload wallets
        await loadWallets();
    } catch (error) {
        alert(`Error creating wallet: ${escapeHtml(error.message)}`);
        console.error('Create wallet error:', error);
    } finally {
        if (createBtn) {
            createBtn.disabled = false;
            createBtn.textContent = '➕ Create New Wallet';
        }
    }
}

async function refreshWalletBalance(address) {
    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/wallet/balance?address=${encodeURIComponent(address)}`);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const data = await response.json();
        if (!data.success) {
            throw new Error(data.error || 'Failed to refresh balance');
        }

        // Reload wallets to show updated balance
        await loadWallets();
    } catch (error) {
        alert(`Error refreshing balance: ${escapeHtml(error.message)}`);
        console.error('Refresh balance error:', error);
    }
}

function formatDate(dateString) {
    if (!dateString) return 'N/A';
    try {
        const date = new Date(dateString);
        return date.toLocaleString();
    } catch (e) {
        return dateString;
    }
}

// Setup event listeners when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    const createWalletBtn = document.getElementById('createWalletBtn');
    const refreshWalletBtn = document.getElementById('refreshWalletBtn');

    if (createWalletBtn) {
        createWalletBtn.addEventListener('click', createWallet);
    }

    if (refreshWalletBtn) {
        refreshWalletBtn.addEventListener('click', () => loadWallets());
    }
});

