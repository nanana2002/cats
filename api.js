// api.js

// ❗ 替换为您的实际 IP 和端口 ❗
const C_SMA_URL = "http://192.168.67.185:8083";
const PLATFORM_URL = "http://192.168.67.185:8080";
const SITE_MAP = {
    "site-1": "http://192.168.235.48:8081", // Linux
    "site-2": "http://192.168.67.159:8085", // Mac
};

document.addEventListener('DOMContentLoaded', () => {
    loadStatus();
    loadServiceList();
    setInterval(loadStatus, 5000); // 每 5 秒刷新站点状态
});

// ===================================
// 1. 站点状态加载 (C-SMA)
// ===================================

async function loadStatus() {
    const statusDiv = document.getElementById('site-status');
    statusDiv.innerHTML = '正在刷新状态...';

    try {
        const response = await fetch(`${C_SMA_URL}/current-metrics`);
        if (!response.ok) throw new Error(`C-SMA返回状态码: ${response.status}`);
        const data = await response.json();

        if (!data.success) throw new Error(`C-SMA业务错误: ${data.message}`);
        
        // 站点数据聚合
        const siteData = {};
        const siteKeys = Object.keys(SITE_MAP);

        // 默认状态为 DOWN
        siteKeys.forEach(key => {
            siteData[key] = {
                status: 'down',
                class: 'status-down',
                metrics: 0,
                total: 'N/A',
                used: 'N/A',
                delay: 'N/A'
            };
        });

        // 尝试从 C-SMA 获取的聚合数据中匹配并更新状态
        if (data.aggregated_data) {
            // 这是一个简化版本，因为它依赖 Site 上的 /resource-status 接口
            // 实际 Site 的可用资源/延迟信息需要通过单独的 API 调用获取 (这里为了简化，使用硬编码)
        }
        
        // 假设 C-SMA 成功，我们尝试从 Site 自己的 /resource-status API 获取资源信息
        const siteMetrics = await Promise.all(siteKeys.map(async (id) => {
            try {
                const res = await fetch(`${SITE_MAP[id]}/resource-status`);
                const json = await res.json();
                if (!json.success) throw new Error('Status check failed');

                const delayRes = await fetch(`${SITE_MAP[id]}/metrics`);
                const delayJson = await delayRes.json();
                
                return {
                    id,
                    status: 'healthy',
                    class: 'status-healthy',
                    metrics: delayJson.count,
                    total: json.resource.total,
                    used: json.resource.used,
                    usage: json.resource.usage_rate,
                    cost: json.cost_conversion,
                    delay: delayJson.metrics.length > 0 ? `${Math.min(...delayJson.metrics.map(m => m.Delay))}ms - ${Math.max(...delayJson.metrics.map(m => m.Delay))}ms` : 'N/A'
                };
            } catch (e) {
                return { id, status: 'down', class: 'status-down', metrics: 0, total: 'N/A' };
            }
        }));

        statusDiv.innerHTML = siteMetrics.map(site => `
            <div class="site-card">
                <h3>${site.id} (${site.id === 'site-1' ? 'Linux' : 'Mac'})</h3>
                <span class="status-badge ${site.class}">${site.status.toUpperCase()}</span>
                <p><strong>实例数:</strong> ${site.metrics}</p>
                <p><strong>总资源:</strong> ${site.total}</p>
                <p><strong>已用/比例:</strong> ${site.used} (${site.usage || 'N/A'})</p>
                <p><strong>成本换算:</strong> ${site.cost || 'N/A'}</p>
                <p><strong>延迟范围:</strong> ${site.delay}</p>
            </div>
        `).join('');

    } catch (e) {
        statusDiv.innerHTML = `<p style="color: red;">无法连接到 C-SMA 或站点: ${e.message}</p>`;
    }
}

// ===================================
// 2. 服务列表加载 (Platform)
// ===================================

async function loadServiceList() {
    try {
        const response = await fetch(`${PLATFORM_URL}/api/v1/services`);
        const data = await response.json();
        
        const serviceSelect = document.getElementById('service-select');
        const siteSelect = document.getElementById('site-select');

        serviceSelect.innerHTML = '';
        siteSelect.innerHTML = '';

        if (data.success && data.services) {
            data.services.forEach(svc => {
                const option = document.createElement('option');
                option.value = svc.ID;
                option.textContent = `${svc.Name} (${svc.ID})`;
                serviceSelect.appendChild(option);
            });
        }

        Object.keys(SITE_MAP).forEach(id => {
            const option = document.createElement('option');
            option.value = id;
            option.textContent = `${id} (${id === 'site-1' ? 'Linux' : 'Mac'})`;
            siteSelect.appendChild(option);
        });

    } catch (e) {
        document.getElementById('deploy-message').textContent = '无法加载服务列表。';
    }
}

// ===================================
// 3. 部署服务 (Site API)
// ===================================

async function deployService() {
    const serviceID = document.getElementById('service-select').value;
    const siteID = document.getElementById('site-select').value;
    const gas = parseInt(document.getElementById('gas-input').value, 10);
    const deployMessage = document.getElementById('deploy-message');
    const logOutput = document.getElementById('log-output');

    if (!serviceID || !siteID || gas < 1) {
        deployMessage.textContent = '请选择服务、站点并输入有效的实例数量。';
        deployMessage.style.color = 'red';
        return;
    }

    const SITE_DEPLOY_URL = `${SITE_MAP[siteID]}/deploy`;

    logOutput.textContent = `[${new Date().toLocaleTimeString()}] 尝试部署 ${serviceID} 到 ${siteID} (${gas}个实例)...`;

    try {
        const response = await fetch(SITE_DEPLOY_URL, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ service_id: serviceID, gas: gas })
        });

        const data = await response.json();

        if (data.success) {
            deployMessage.textContent = `✅ 部署成功! ${data.message}`;
            deployMessage.style.color = 'green';
            logOutput.textContent += `\n[SUCCESS] ${data.info.CSCI_ID} (Cost: ${data.info.Cost}, Delay: ${data.info.Delay}ms)`;
        } else {
            deployMessage.textContent = `❌ 部署失败: ${data.message}`;
            deployMessage.style.color = 'red';
            logOutput.textContent += `\n[FAILED] ${data.message}`;
        }
        logOutput.scrollTop = logOutput.scrollHeight;
        loadStatus(); // 部署后刷新状态

    } catch (e) {
        deployMessage.textContent = `致命错误: 无法连接到 ${siteID} 部署接口。`;
        deployMessage.style.color = 'red';
        logOutput.textContent += `\n[FATAL] 连接错误: ${e.message}`;
        logOutput.scrollTop = logOutput.scrollHeight;
    }
}