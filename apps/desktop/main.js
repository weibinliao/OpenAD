import { invoke } from '@tauri-apps/api/tauri';

const desktopWebPort = '3010';

// Redirect to web interface
window.location.href = `http://localhost:${desktopWebPort}`;

// Desktop-specific functions available globally
window.selectFolder = async () => {
    try {
        return await invoke('select_folder');
    } catch (error) {
        console.error('Failed to select folder:', error);
        return null;
    }
};

window.scanPermissions = async (path) => {
    try {
        return await invoke('scan_permissions', { path });
    } catch (error) {
        console.error('Failed to scan permissions:', error);
        return null;
    }
};
