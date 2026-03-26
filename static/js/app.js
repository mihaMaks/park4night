let deleteRecordId = null;
let timelineChart = null;
let forecastCharts = [];
let selectedStates = new Set();
let availableStates = [];
let showAllRecords = true;
let currentPage = 1;
let pageSize = 50;
let totalRecords = 0;

document.addEventListener('DOMContentLoaded', () => {
    // Load records and chart on page load
    loadRecords();
    loadStates();
    loadChart();
    loadForecast();

    // Form submission
    document.getElementById('recordForm').addEventListener('submit', handleFormSubmit);
    document.getElementById('clearBtn').addEventListener('click', clearForm);

    // Forecast
    document.getElementById('forecastBtn').addEventListener('click', loadForecast);

    // Table filters
    document.getElementById('searchBtn').addEventListener('click', () => {
        currentPage = 1;
        loadRecords();
    });
    document.getElementById('resetBtn').addEventListener('click', resetFilters);

    // Pagination
    document.getElementById('prevPage').addEventListener('click', () => {
        if (currentPage > 1) {
            currentPage--;
            loadRecords();
        }
    });
    document.getElementById('nextPage').addEventListener('click', () => {
        currentPage++;
        loadRecords();
    });

    // Chart filters
    document.getElementById('updateChartBtn').addEventListener('click', loadChart);
    document.getElementById('resetChartBtn').addEventListener('click', resetChartFilters);

    // Modal actions
    document.getElementById('confirmDeleteBtn').addEventListener('click', confirmDelete);
    document.getElementById('cancelDeleteBtn').addEventListener('click', closeDeleteModal);
});

function loadStates() {
    fetch('/api/states')
        .then(response => response.json())
        .then(data => {
            availableStates = data.states;
            createStateButtons();
        })
        .catch(error => {
            console.error('Error loading states:', error);
        });
}

function hexToRgba(hex, alpha) {
    const sanitized = hex.replace('#', '');
    const bigint = parseInt(sanitized, 16);
    const r = (bigint >> 16) & 255;
    const g = (bigint >> 8) & 255;
    const b = bigint & 255;
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

function createStateButtons() {
    const container = document.querySelector('.state-buttons');
    const allBtn = document.getElementById('allStatesBtn');

    // Reset container and re-add the "All Records" button
    container.innerHTML = '';
    container.appendChild(allBtn);

    // Add buttons for each state
    availableStates.forEach(state => {
        const button = document.createElement('button');
        button.className = 'state-btn';
        button.dataset.state = state;
        button.textContent = state;
        button.addEventListener('click', () => toggleState(state));
        container.appendChild(button);
    });

    // Setup All Records button behavior (toggle)
    allBtn.addEventListener('click', () => {
        showAllRecords = !showAllRecords;
        updateStateButtonStyles();
        loadChart();
    });

    // Initialize button styling
    updateStateButtonStyles();
}

function toggleState(state) {
    if (selectedStates.has(state)) {
        selectedStates.delete(state);
    } else {
        selectedStates.add(state);
    }

    updateStateButtonStyles();
    loadChart();
}

function updateStateButtonStyles() {
    const buttons = document.querySelectorAll('.state-btn');
    buttons.forEach(btn => {
        const state = btn.dataset.state;
        if (state === 'all') {
            btn.classList.toggle('active', showAllRecords);
        } else {
            btn.classList.toggle('active', selectedStates.has(state));
        }
    });
}

function loadRecords() {
    const dateFilter = document.getElementById('filterDate').value;
    const stateFilter = document.getElementById('filterState').value;
    const adultsFilter = document.getElementById('filterAdults').value;
    const childrenFilter = document.getElementById('filterChildren').value;
    const bicyclesFilter = document.getElementById('filterBicycles').value;

    const offset = (currentPage - 1) * pageSize;
    let url = `/api/records?limit=${pageSize}&offset=${offset}`;
    if (dateFilter) url += `&date=${encodeURIComponent(dateFilter)}`;
    if (stateFilter) url += `&state=${encodeURIComponent(stateFilter)}`;
    if (adultsFilter && adultsFilter !== '0') url += `&minAdults=${encodeURIComponent(adultsFilter)}`;
    if (childrenFilter && childrenFilter !== '0') url += `&minChildren=${encodeURIComponent(childrenFilter)}`;
    if (bicyclesFilter && bicyclesFilter !== '0') url += `&minBicycles=${encodeURIComponent(bicyclesFilter)}`;

    fetch(url)
        .then(response => response.json())
        .then(data => {
            if (!data || data.error) {
                const message = (data && data.error) ? data.error : 'No records found';
                populateTable([]);
                updatePagination(0);
                showToast(message, 'info');
                return;
            }

            const records = Array.isArray(data.data) ? data.data : [];
            totalRecords = data.total || 0;

            populateTable(records);
            updatePagination(totalRecords);
        })
        .catch(error => {
            console.error('Error loading records:', error);
            showToast('Failed to load records', 'error');
        });
}

function updatePagination(total) {
    const totalPages = Math.ceil(total / pageSize);
    const pageInfo = document.getElementById('pageInfo');
    const prevBtn = document.getElementById('prevPage');
    const nextBtn = document.getElementById('nextPage');
    const recordCount = document.getElementById('recordCount');

    pageInfo.textContent = `Page ${currentPage} of ${totalPages || 1}`;
    recordCount.textContent = `Total: ${total} records`;

    prevBtn.disabled = currentPage <= 1;
    nextBtn.disabled = currentPage >= totalPages || totalPages === 0;
}

function loadChart() {
    const dateFrom = document.getElementById('chartDateFrom').value;
    const dateTo = document.getElementById('chartDateTo').value;
    const minChildren = document.getElementById('chartChildrenFilter').value;
    const minBicycles = document.getElementById('chartBicyclesFilter').value;

    // Build base params - no state filter on main request, only date and quantity filters
    const baseParams = [];
    if (dateFrom) baseParams.push(`dateFrom=${encodeURIComponent(dateFrom)}`);
    if (dateTo) baseParams.push(`dateTo=${encodeURIComponent(dateTo)}`);
    if (minChildren && minChildren !== '0') baseParams.push(`minChildren=${minChildren}`);
    if (minBicycles && minBicycles !== '0') baseParams.push(`minBicycles=${minBicycles}`);

    // Request all-data if enabled
    const promises = [];
    let allData = [];
    let allTooltips = {};
    if (showAllRecords) {
        const allUrl = '/api/chart-data' + (baseParams.length ? `?${baseParams.join('&')}` : '');
        promises.push(fetch(allUrl).then(r => r.json()));
    }

    // Request each selected state separately (if any)
    const stateRequests = [];
    selectedStates.forEach(state => {
        const params = [...baseParams, `state=${encodeURIComponent(state)}`];
        const stateUrl = '/api/chart-data' + (params.length ? `?${params.join('&')}` : '');
        stateRequests.push(fetch(stateUrl).then(r => r.json()).then(res => ({ state, data: res.data, tooltips: res.tooltips })));
    });

    Promise.all([...promises, ...stateRequests])
        .then(results => {
            if (showAllRecords) {
                allData = results[0].data;
                allTooltips = results[0].tooltips || {};
                results = results.slice(1);
            }
            const stateData = results.map(r => ({ state: r.state, data: r.data, tooltips: r.tooltips || {} }));
            renderChart(allData, stateData, allTooltips);
        })
        .catch(error => {
            console.error('Error loading chart data:', error);
            showToast('Failed to load chart data', 'error');
        });
}

function loadForecast() {
    const params = [];

    const url = '/api/forecast' + (params.length ? `?${params.join('&')}` : '');

    fetch(url)
        .then(response => response.json())
        .then(data => {
            renderForecastChart(data);
        })
        .catch(error => {
            console.error('Error loading forecast:', error);
            showToast('Failed to load forecast', 'error');
        });
}

function renderForecastChart(data) {
    const colors = [
        '#080d22',
        '#34d399',
        '#f59e0b',
        '#ec4899',
        '#14b8a6',
        '#f97316',
        '#22c55e'
    ];

    // Destroy existing charts
    forecastCharts.forEach(chart => {
        if (chart) chart.destroy();
    });
    forecastCharts = [];

    for (let i = 0; i < 3; i++) {
        const year = (data.years || [])[i];
        const chartDiv = document.getElementById('forecastChart' + (i + 1));
        const yearHeader = document.getElementById('year' + (i + 1));

        if (!year) {
            if (chartDiv) chartDiv.style.display = 'none';
            if (yearHeader) yearHeader.style.display = 'none';
            continue;
        }

        if (chartDiv) chartDiv.style.display = 'block';
        if (yearHeader) yearHeader.style.display = 'block';

        const ctx = chartDiv.getContext('2d');
        const counts = (data.counts && data.counts[year]) || [];

        const dataset = {
            label: year.toString(),
            data: counts,
            borderColor: colors[i % colors.length],
            backgroundColor: hexToRgba(colors[i % colors.length], 0.6),
            borderWidth: 1
        };

        forecastCharts[i] = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: data.labels || [],
                datasets: [dataset]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            stepSize: 1
                        }
                    }
                },
                plugins: {
                    legend: {
                        display: false
                    },
                    tooltip: {
                        callbacks: {
                            title: (context) => context[0]?.label || '',
                            label: (context) => `${context.parsed.y} records`
                        }
                    }
                }
            }
        });

        if (yearHeader) yearHeader.textContent = year;
    }
}

function renderChart(allData, selectedStatesData = [], allTooltips = {}) {
    const ctx = document.getElementById('timelineChart').getContext('2d');
    
    // Destroy existing chart if it exists
    if (timelineChart) {
        timelineChart.destroy();
    }

    // Merge all tooltips from all data sources
    const recordDetailsByDate = {};
    
    // Add tooltips from all records
    Object.assign(recordDetailsByDate, allTooltips);
    
    // Add tooltips from selected states
    selectedStatesData.forEach(entry => {
        const stateTooltips = entry.tooltips || {};
        Object.keys(stateTooltips).forEach(date => {
            if (!recordDetailsByDate[date]) {
                recordDetailsByDate[date] = [];
            }
            recordDetailsByDate[date] = recordDetailsByDate[date].concat(stateTooltips[date] || []);
        });
    });

    // Convert allData into map by date for faster lookup
    const allMap = new Map();
    allData.forEach(point => {
        allMap.set(point.date, point.count);
    });

    // Determine full date range (min..max) based on available data
    let dates = Array.from(allMap.keys());

    // If no all-data (when hidden), derive range from selected states
    if (dates.length === 0 && selectedStatesData.length > 0) {
        selectedStatesData.forEach(entry => {
            entry.data.forEach(point => {
                dates.push(point.date);
            });
        });
    }

    dates = Array.from(new Set(dates)).sort();
    if (dates.length === 0) {
        // No data to show
        timelineChart = new Chart(ctx, {
            type: 'line',
            data: { labels: [], datasets: [] },
            options: { responsive: true, maintainAspectRatio: false }
        });
        return;
    }

    const firstDate = new Date(dates[0]);
    const lastDate = new Date(dates[dates.length - 1]);

    // Build full date range including missing days
    const allDates = [];
    for (let d = new Date(firstDate); d <= lastDate; d.setDate(d.getDate() + 1)) {
        const iso = d.toISOString().slice(0, 10);
        allDates.push(iso);
    }

    const labels = allDates.map(d => {
        const date = new Date(d);
        return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    });

    const datasets = [];

    // Helper to format tooltip details from server-formatted data
    function formatTooltipDetails(isoDate) {
        const details = recordDetailsByDate[isoDate] || [];
        if (!Array.isArray(details)) return [];
        
        return details.map(detail => {
            return `${detail.state}: ${detail.count} record(s) (A:${detail.adults} C:${detail.children} B:${detail.bicycles})`;
        });
    }

    if (showAllRecords) {
        const allCounts = allDates.map(d => allMap.get(d) || 0);
        datasets.push({
            label: 'All Records',
            data: allCounts,
            borderColor: '#667eea',
            backgroundColor: 'rgba(102, 126, 234, 0.1)',
            borderWidth: 2,
            fill: false,
            tension: 0.1
        });
    }

    const colors = [
        '#ff6b6b',
        '#34d399',
        '#f59e0b',
        '#6366f1',
        '#ec4899',
        '#14b8a6',
        '#f97316',
        '#22c55e'
    ];

    selectedStatesData.forEach((entry, idx) => {
        const stateMap = new Map();
        entry.data.forEach(point => {
            stateMap.set(point.date, point.count);
        });

        const counts = allDates.map(d => stateMap.get(d) || 0);
        const color = colors[idx % colors.length];

        datasets.push({
            label: `${entry.state} Records`,
            data: counts,
            borderColor: color,
            backgroundColor: hexToRgba(color, 0.1),
            borderWidth: 2,
            fill: false,
            tension: 0.1
        });
    });

    timelineChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: datasets
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1
                    }
                }
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top'
                },
                tooltip: {
                    callbacks: {
                        afterBody: (context) => {
                            if (!context || context.length === 0) return [];
                            const idx = context[0].dataIndex;
                            const isoDate = allDates[idx];
                            return formatTooltipDetails(isoDate);
                        }
                    }
                }
            }
        }
    });
}

function resetChartFilters() {
    document.getElementById('chartDateFrom').value = '';
    document.getElementById('chartDateTo').value = '';
    document.getElementById('chartChildrenFilter').value = '0';
    document.getElementById('chartBicyclesFilter').value = '0';

    // Reset state selection to show all records
    selectedStates.clear();
    showAllRecords = true;
    updateStateButtonStyles();

    loadChart();
}

function populateTable(records) {
    const tbody = document.getElementById('tableBody');
    tbody.innerHTML = '';

    if (records.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="loading">No records found</td></tr>';
        return;
    }

    records.forEach(record => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${record.id}</td>
            <td>${record.date || '-'}</td>
            <td>${record.state || '-'}</td>
            <td>${record.adults || 0}</td>
            <td>${record.children || 0}</td>
            <td>${record.bicycles || 0}</td>
            <td>
                <div class="action-buttons">
                    <button class="btn-edit" onclick="editRecord(${record.id})">Edit</button>
                    <button class="btn-delete" onclick="confirmDeleteRecord(${record.id})">Delete</button>
                </div>
            </td>
        `;
        tbody.appendChild(row);
    });
}

function handleFormSubmit(e) {
    e.preventDefault();

    const recordId = document.getElementById('recordId').value;
    const date = document.getElementById('date').value;
    const state = document.getElementById('state').value;
    const adults = parseInt(document.getElementById('adults').value) || 0;
    const children = parseInt(document.getElementById('children').value) || 0;
    const bicycles = parseInt(document.getElementById('bicycles').value) || 0;

    if (!date || !state) {
        showToast('Please fill in all required fields', 'error');
        return;
    }

    const data = { date, state, adults, children, bicycles };
    const method = recordId ? 'PUT' : 'POST';
    const url = recordId ? `/api/records/${recordId}` : '/api/records';

    fetch(url, {
        method: method,
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data)
    })
        .then(response => response.json())
        .then(result => {
            const message = recordId ? 'Record updated successfully' : 'Record created successfully';
            showToast(message, 'success');
            clearForm();
            loadRecords();
        })
        .catch(error => {
            console.error('Error:', error);
            showToast('Failed to save record', 'error');
        });
}

function clearForm() {
    document.getElementById('recordForm').reset();
    document.getElementById('recordId').value = '';
    document.getElementById('submitBtn').textContent = 'Add Record';
}

function editRecord(id) {
    fetch(`/api/records/${id}`)
        .then(response => response.json())
        .then(record => {
            document.getElementById('recordId').value = record.id;
            document.getElementById('date').value = record.date;
            document.getElementById('state').value = record.state;
            document.getElementById('adults').value = record.adults || 0;
            document.getElementById('children').value = record.children || 0;
            document.getElementById('bicycles').value = record.bicycles || 0;
            document.getElementById('submitBtn').textContent = 'Update Record';
            
            // Scroll to form
            document.querySelector('.form-section').scrollIntoView({ behavior: 'smooth' });
        })
        .catch(error => {
            console.error('Error loading record:', error);
            showToast('Failed to load record', 'error');
        });
}

function confirmDeleteRecord(id) {
    deleteRecordId = id;
    document.getElementById('deleteModal').classList.add('show');
}

function confirmDelete() {
    if (!deleteRecordId) return;

    fetch(`/api/records/${deleteRecordId}`, {
        method: 'DELETE',
    })
        .then(response => response.json())
        .then(result => {
            showToast('Record deleted successfully', 'success');
            closeDeleteModal();
            loadRecords();
        })
        .catch(error => {
            console.error('Error:', error);
            showToast('Failed to delete record', 'error');
        });
}

function closeDeleteModal() {
    document.getElementById('deleteModal').classList.remove('show');
    deleteRecordId = null;
}

function resetFilters() {
    document.getElementById('filterDate').value = '';
    document.getElementById('filterState').value = '';
    document.getElementById('filterAdults').value = '0';
    document.getElementById('filterChildren').value = '0';
    document.getElementById('filterBicycles').value = '0';
    currentPage = 1;
    loadRecords();
}

function showToast(message, type = 'success') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast show ${type}`;
    
    // Auto hide after 3 seconds
    setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

// Close modal when clicking outside
document.getElementById('deleteModal').addEventListener('click', (e) => {
    if (e.target === document.getElementById('deleteModal')) {
        closeDeleteModal();
    }
});
