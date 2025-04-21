// 全局变量
let currentPage = 1;
let currentSortField = 'id';
let currentSortOrder = 'desc';

// 加载配置列表
function loadConfigs(page, keyword = '', method = '') {
    currentPage = page;
    
    $.get('/api/list', {
        page: page,
        pageSize: 10,
        keyword: keyword || $('#keyword').val(),
        method: method || $('#method-filter').val(),
        sortField: currentSortField,
        sortOrder: currentSortOrder
    }, function(data) {
        if(data.code === 0) {
            renderConfigTable(data.data.data);
            updatePagination(data.data.total, data.data.totalPages);
        }
    });
}

// 渲染配置表格
function renderConfigTable(configs) {
    const $tableBody = $('#config-table');
    $tableBody.empty();
    
    if(configs.length === 0) {
        $tableBody.append('<tr><td colspan="5" class="text-center">没有找到配置</td></tr>');
        return;
    }
    
    configs.forEach(config => {
        $tableBody.append(`
            <tr>
                <td>${config.id}</td>
                <td><a href="/detail?id=${config.id}">${config.name}</a></td>
                <td>${config.url}</td>
                <td><span class="method-badge ${config.method.toLowerCase()}">${config.method}</span></td>
                <td>
                    <a href="/edit?id=${config.id}" class="btn btn-sm"><i class="fa fa-edit"></i></a>
                    <button class="btn btn-sm btn-danger delete-btn" data-id="${config.id}">
                        <i class="fa fa-trash"></i>
                    </button>
                </td>
            </tr>
        `);
    });
    
    // 绑定删除按钮事件
    $('.delete-btn').click(function() {
        const configId = $(this).data('id');
        if(confirm('确定要删除此配置吗？')) {
            $.ajax({
                url: `/api/delete`,
                type: 'POST',
                data: { id: configId },
                success: function() {
                    loadConfigs(currentPage);
                }
            });
        }
    });
}

// 更新分页控件
function updatePagination(total, totalPages) {
    $('#page-info').text(`第 ${currentPage} 页 / 共 ${totalPages} 页 (${total} 条)`);
    $('#prev-page').prop('disabled', currentPage <= 1);
    $('#next-page').prop('disabled', currentPage >= totalPages);
}

// 排序功能
function setupSorting() {
    $('th').click(function() {
        const field = $(this).data('field');
        if(field) {
            if(currentSortField === field) {
                currentSortOrder = currentSortOrder === 'asc' ? 'desc' : 'asc';
            } else {
                currentSortField = field;
                currentSortOrder = 'asc';
            }
            loadConfigs(1);
        }
    });
}

// 初始化
$(document).ready(function() {
    setupSorting();
    
    // 搜索框回车事件
    $('#keyword').keypress(function(e) {
        if(e.which === 13) {
            loadConfigs(1);
        }
    });
    
    // 方法筛选变更事件
    $('#method-filter').change(function() {
        loadConfigs(1);
    });
});