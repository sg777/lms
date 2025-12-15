// Authentication functions

async function checkAuth() {
    if (!authToken) return false;
    
    try {
        const response = await fetch(`${API_BASE}/api/auth/me`, {
            headers: {
                'Authorization': `Bearer ${authToken}`
            }
        });
        
        if (!response.ok) {
            localStorage.removeItem('auth_token');
            authToken = null;
            updateAuthUI();
            return false;
        }
        
        const data = await response.json();
        if (data.success && data.user) {
            currentUser = data.user;
            updateAuthUI();
            if (document.getElementById('myKeysTab')) {
                loadMyKeys();
            }
            return true;
        }
    } catch (error) {
        console.error('Auth check failed:', error);
    }
    
    return false;
}

async function handleLogin(e) {
    e.preventDefault();
    const username = document.getElementById('loginUsername').value;
    const password = document.getElementById('loginPassword').value;
    const errorDiv = document.getElementById('loginError');
    
    errorDiv.style.display = 'none';
    
    try {
        const response = await fetch(`${API_BASE}/api/auth/login`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, password })
        });
        
        const data = await response.json();
        
        if (data.success && data.token) {
            authToken = data.token;
            currentUser = data.user;
            localStorage.setItem('auth_token', authToken);
            updateAuthUI();
            closeModal('loginModal');
            document.getElementById('loginForm').reset();
            loadMyKeys();
        } else {
            errorDiv.textContent = data.error || 'Login failed';
            errorDiv.style.display = 'block';
        }
    } catch (error) {
        errorDiv.textContent = 'Network error: ' + error.message;
        errorDiv.style.display = 'block';
    }
}

async function handleRegister(e) {
    e.preventDefault();
    const username = document.getElementById('registerUsername').value;
    const email = document.getElementById('registerEmail').value;
    const password = document.getElementById('registerPassword').value;
    const errorDiv = document.getElementById('registerError');
    
    errorDiv.style.display = 'none';
    
    try {
        const response = await fetch(`${API_BASE}/api/auth/register`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, email, password })
        });
        
        const data = await response.json();
        
        if (data.success && data.token) {
            authToken = data.token;
            currentUser = data.user;
            localStorage.setItem('auth_token', authToken);
            updateAuthUI();
            closeModal('registerModal');
            document.getElementById('registerForm').reset();
            loadMyKeys();
        } else {
            errorDiv.textContent = data.error || 'Registration failed';
            errorDiv.style.display = 'block';
        }
    } catch (error) {
        errorDiv.textContent = 'Network error: ' + error.message;
        errorDiv.style.display = 'block';
    }
}

function handleLogout() {
    authToken = null;
    currentUser = null;
    localStorage.removeItem('auth_token');
    updateAuthUI();
    
    // Switch to explorer tab if on my keys
    if (document.getElementById('myKeysTabContent').classList.contains('active')) {
        switchTab('explorer');
    }
}

function updateAuthUI() {
    const loginSection = document.getElementById('loginSection');
    const userInfo = document.getElementById('userInfo');
    const usernameDisplay = document.getElementById('usernameDisplay');
    const myKeysTab = document.getElementById('myKeysTab');
    const walletTab = document.getElementById('walletTab');
    
    if (authToken && currentUser) {
        loginSection.style.display = 'none';
        userInfo.style.display = 'block';
        usernameDisplay.textContent = currentUser.username;
        if (myKeysTab) {
            myKeysTab.style.display = 'inline-block';
        }
        if (walletTab) {
            walletTab.style.display = 'inline-block';
        }
    } else {
        loginSection.style.display = 'block';
        userInfo.style.display = 'none';
        if (myKeysTab) {
            myKeysTab.style.display = 'none';
        }
        if (walletTab) {
            walletTab.style.display = 'none';
        }
        // Clear my keys list
        const myKeysList = document.getElementById('myKeysList');
        if (myKeysList) {
            myKeysList.innerHTML = '<div class="loading">Please login to view your keys</div>';
        }
    }
}

function openModal(modalId) {
    document.getElementById(modalId).style.display = 'block';
}

function closeModal(modalId) {
    document.getElementById(modalId).style.display = 'none';
    // Clear errors
    const errorDivs = document.getElementById(modalId).querySelectorAll('.error-message');
    errorDivs.forEach(div => {
        div.style.display = 'none';
        div.textContent = '';
    });
}

function switchTab(tabName) {
    // Update tab buttons
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.remove('active');
        if (btn.getAttribute('data-tab') === tabName) {
            btn.classList.add('active');
        }
    });
    
    // Update tab content
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
        content.style.display = 'none';
    });
    
    let targetTab = 'explorerTab';
    if (tabName === 'mykeys') {
        targetTab = 'myKeysTabContent';
    } else if (tabName === 'wallet') {
        targetTab = 'walletTabContent';
    }
    
    const targetElement = document.getElementById(targetTab);
    if (targetElement) {
        targetElement.classList.add('active');
        targetElement.style.display = 'block';
    }
    
    // Load data if switching to my keys or wallet
    if (tabName === 'mykeys' && authToken) {
        loadMyKeys();
    } else if (tabName === 'wallet' && authToken) {
        if (typeof loadWallets === 'function') {
            loadWallets();
        }
    }
}

// Helper to make authenticated API calls
async function authenticatedFetch(url, options = {}) {
    const headers = {
        'Content-Type': 'application/json',
        ...options.headers
    };
    
    if (authToken) {
        headers['Authorization'] = `Bearer ${authToken}`;
    }
    
    return fetch(url, {
        ...options,
        headers
    });
}

