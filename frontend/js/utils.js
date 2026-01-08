// === Utility Functions ===

export const Utils = {
    // Format category name with emoji
    formatCategoryName(name) {
        const names = {
            'domains': '🌐 Domains',
            'security': '🔒 Security',
            'recon': '🔍 Recon',
            'infrastructure': '⚙️ Infrastructure',
            'web_analysis': '📊 Web Analysis'
        };
        return names[name] || name;
    },

    // Get response time class for color-coding
    getResponseTimeClass(ms) {
        if (!ms) return '';
        if (ms < 100) return 'fast';
        if (ms < 500) return 'medium';
        return 'slow';
    },

    // Format time ago
    timeAgo(date) {
        if (!date) return '--';
        const now = new Date();
        const diff = Math.floor((now - new Date(date)) / 1000);

        if (diff < 60) return `${diff}s ago`;
        if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
        return `${Math.floor(diff / 86400)}d ago`;
    },

    // Format time
    formatTime(date) {
        if (!date) return '--';
        return new Date(date).toLocaleTimeString();
    },

    // Debounce function
    debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    },

    // Escape HTML
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
};

export default Utils;
