const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

class LinuxPermissionScanner {
    async scanFolder(folderPath, maxDepth = 2) {
        console.log(`Scanning: ${folderPath}`);

        try {
            // Use ls -la for detailed permissions
            const cmd = `find "${folderPath}" -maxdepth ${maxDepth} -exec ls -la {} \\;`;
            const result = execSync(cmd, { encoding: 'utf8', timeout: 10000 });
            return this.parseResults(result);
        } catch (error) {
            console.error('Scan failed:', error.message);
            return this.fallbackScan(folderPath, maxDepth);
        }
    }

    parseResults(output) {
        const lines = output.split('\n').filter(line => line.trim());
        const permissions = [];

        lines.forEach(line => {
            const parts = line.split(/\s+/);
            if (parts.length >= 9) {
                permissions.push({
                    permissions: parts[0],
                    owner: parts[2],
                    group: parts[3],
                    size: parts[4],
                    path: parts.slice(8).join(' '),
                    platform: 'linux'
                });
            }
        });

        return permissions;
    }

    fallbackScan(folderPath, maxDepth) {
        const permissions = [];

        const traverse = (dir, depth) => {
            if (depth > maxDepth) return;

            try {
                const items = fs.readdirSync(dir, { withFileTypes: true });

                items.forEach(item => {
                    const fullPath = path.join(dir, item.name);
                    try {
                        const stats = fs.statSync(fullPath);
                        permissions.push({
                            path: fullPath,
                            isDirectory: item.isDirectory(),
                            size: stats.size,
                            mode: stats.mode.toString(8),
                            platform: 'linux'
                        });

                        if (item.isDirectory() && depth < maxDepth) {
                            traverse(fullPath, depth + 1);
                        }
                    } catch (e) {
                        permissions.push({
                            path: fullPath,
                            error: 'Access denied',
                            platform: 'linux'
                        });
                    }
                });
            } catch (error) {
                console.error(`Cannot read: ${dir}`);
            }
        };

        traverse(folderPath, 0);
        return permissions;
    }

    exportToJson(permissions, outputPath) {
        const report = {
            scanDate: new Date().toISOString(),
            platform: 'linux',
            totalItems: permissions.length,
            permissions: permissions
        };

        fs.writeFileSync(outputPath, JSON.stringify(report, null, 2));
        console.log(`Report saved: ${outputPath}`);
        return report;
    }
}

module.exports = LinuxPermissionScanner;
