// === API Service Layer ===
// All API calls in one place with error handling

const API = {
    baseUrl: '',

    async get(endpoint) {
        try {
            const response = await fetch(`${this.baseUrl}${endpoint}`);
            if (!response.ok) throw new Error(`HTTP ${response.status}`);
            return await response.json();
        } catch (error) {
            console.error(`API GET ${endpoint} failed:`, error);
            throw error;
        }
    },

    async post(endpoint) {
        try {
            const response = await fetch(`${this.baseUrl}${endpoint}`, { method: 'POST' });
            if (!response.ok) throw new Error(`HTTP ${response.status}`);
            return await response.json();
        } catch (error) {
            console.error(`API POST ${endpoint} failed:`, error);
            throw error;
        }
    },

    // Specific API methods
    async getServices() {
        return this.get('/api/services');
    },

    async getStats() {
        return this.get('/api/stats');
    },

    async getCategories() {
        return this.get('/api/categories');
    },

    async testService(id) {
        return this.post(`/api/test/${id}`);
    },

    async testCategory(category) {
        return this.post(`/api/test-category/${category}`);
    },

    async refresh() {
        return this.post('/api/refresh');
    }
};

export default API;
