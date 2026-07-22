// Auto-refresh build status every 10 seconds
(function() {
    'use strict';

    // Highlight active nav link
    document.querySelectorAll('.nav-link').forEach(function(link) {
        var href = link.getAttribute('href');
        if (href && window.location.pathname.startsWith(href)) {
            link.classList.add('active');
        }
    });

    // Toast notification
    function showToast(msg, duration) {
        var toast = document.getElementById('toast');
        if (!toast) return;
        toast.textContent = msg;
        toast.style.display = 'block';
        setTimeout(function() { toast.style.display = 'none'; }, duration || 3000);
    }
    window.showToast = showToast;

    // Build log modal
    window.showBuildLog = function(buildId) {
        var modal = document.getElementById('logModal');
        var body = document.getElementById('logModalBody');
        if (!modal) return;
        body.textContent = '加载中...';
        modal.style.display = 'flex';
        fetch('/builds/' + buildId + '/log')
            .then(function(r) {
                if (!r.ok) throw new Error('HTTP ' + r.status);
                return r.text();
            })
            .then(function(text) {
                body.textContent = text || '(无日志内容)';
            })
            .catch(function(e) {
                body.textContent = '加载失败: ' + e.message;
            });
    };

    // Manifest modal
    window.showManifest = function(releaseId) {
        var modal = document.getElementById('manifestModal');
        var body = document.getElementById('manifestModalBody');
        if (!modal) return;
        body.textContent = '加载中...';
        modal.style.display = 'flex';
        fetch('/api/releases/' + releaseId + '/manifest')
            .then(function(r) {
                if (!r.ok) throw new Error('HTTP ' + r.status);
                return r.json();
            })
            .then(function(data) {
                body.textContent = JSON.stringify(data, null, 2);
            })
            .catch(function(e) {
                body.textContent = '加载失败: ' + e.message;
            });
    };

    // Artifacts modal
    window.showArtifacts = function(buildId) {
        var modal = document.getElementById('artifactModal');
        var body = document.getElementById('artifactModalBody');
        if (!modal) return;
        modal.style.display = 'flex';
        body.innerHTML = '<div style="text-align:center;color:#94a3b8">加载中...</div>';
        fetch('/builds/' + buildId + '/artifacts')
            .then(function(r) { return r.json(); })
            .then(function(artifacts) {
                if (!artifacts || artifacts.length === 0) {
                    body.innerHTML = '<div style="text-align:center;color:#94a3b8;padding:20px">暂无制品</div>';
                    return;
                }
                var html = '<table class="table" style="margin:0"><thead><tr><th>文件名</th><th>组件</th><th>大小</th><th>操作</th></tr></thead><tbody>';
                artifacts.forEach(function(a) {
                    var size = a.file_size < 1024 ? a.file_size + ' B'
                        : a.file_size < 1048576 ? (a.file_size / 1024).toFixed(1) + ' KB'
                        : (a.file_size / 1048576).toFixed(1) + ' MB';
                    html += '<tr><td><code>' + a.file_name + '</code></td>'
                        + '<td>' + (a.component_name || '-') + '</td>'
                        + '<td>' + size + '</td>'
                        + '<td><a href="/builds/artifacts/' + a.id + '/download" class="btn btn-sm btn-download">⬇ 下载</a></td></tr>';
                });
                html += '</tbody></table>';
                body.innerHTML = html;
            })
            .catch(function(e) {
                body.innerHTML = '<div style="text-align:center;color:#ef4444">加载失败: ' + e.message + '</div>';
            });
    };

    // Delete release from expandable list (AJAX, no page reload)
    window.deleteReleaseFromList = function(btn, releaseId) {
        var modal = document.getElementById('confirmModal');
        var msgEl = document.getElementById('confirmModalMsg');
        var okBtn = document.getElementById('confirmOk');
        var cancelBtn = document.getElementById('confirmCancel');
        if (!modal) return;
        msgEl.textContent = '确定删除此版本发布记录？';
        modal.style.display = 'flex';

        function closeConfirm() {
            modal.style.display = 'none';
            okBtn.replaceWith(okBtn.cloneNode(true));
            cancelBtn.replaceWith(cancelBtn.cloneNode(true));
        }

        document.getElementById('confirmCancel').onclick = closeConfirm;
        document.getElementById('confirmOk').onclick = function() {
            closeConfirm();
            btn.disabled = true;
            btn.textContent = '删除中...';
            fetch('/releases/' + releaseId + '/delete', {
                method: 'POST',
                redirect: 'manual'
            }).then(function(resp) {
                if (resp.type === 'opaqueredirect' || resp.ok) {
                    var tr = btn.closest('tr[data-release-id]');
                    if (tr) tr.remove();
                    // Check if list is now empty
                    var tbody = btn.closest('tbody');
                    if (tbody && tbody.children.length === 0) {
                        var expandRow = btn.closest('.row-expand');
                        if (expandRow) {
                            var empty = expandRow.querySelector('.expand-empty');
                            var table = expandRow.querySelector('.table-nested');
                            if (empty) empty.style.display = '';
                            if (table) table.style.display = 'none';
                        }
                    }
                    showToast('✅ 已删除', 2000);
                } else {
                    btn.disabled = false;
                    btn.textContent = '删除';
                    showToast('❌ 删除失败', 3000);
                }
            }).catch(function(err) {
                btn.disabled = false;
                btn.textContent = '删除';
                showToast('❌ 删除失败: ' + err.message, 3000);
            });
        };
    };

    // Description modal
    window.showDescription = function(el) {
        var modal = document.getElementById('descModal');
        var body = document.getElementById('descModalBody');
        if (!modal) return;
        var text = el.getAttribute('data-full') || '';
        // Render as HTML if it contains tags (from Quill editor), else as plain text
        if (text.indexOf('<') !== -1) {
            body.innerHTML = text;
        } else {
            body.textContent = text;
        }
        modal.style.display = 'flex';
    };

    // HTML escape helper for data attributes
    function escapeAttr(s) {
        if (!s) return '';
        return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
    // Strip HTML tags for plain text preview
    function stripHtml(html) {
        if (!html) return '';
        var tmp = document.createElement('div');
        tmp.innerHTML = html;
        return tmp.textContent || tmp.innerText || '';
    }

    // Release components modal
    window.showReleaseComps = function(el) {
        var releaseId = el.getAttribute('data-release-id');
        var modal = document.getElementById('compsModal');
        var body = document.getElementById('compsModalBody');
        if (!modal) return;
        body.innerHTML = '<div style="text-align:center;color:#94a3b8">加载中...</div>';
        modal.style.display = 'flex';
        fetch('/api/releases/' + releaseId + '/manifest')
            .then(function(r) { return r.json(); })
            .then(function(data) {
                // Try to extract components from manifest
                var comps = data.components || data;
                if (Array.isArray(comps) && comps.length > 0) {
                    var html = '<table class="table" style="margin:0"><thead><tr><th>组件名称</th><th>版本</th><th>分支</th><th>制品</th></tr></thead><tbody>';
                    comps.forEach(function(c) {
                        html += '<tr><td><strong>' + (c.component_name || c.name || '-') + '</strong></td>'
                            + '<td><code>' + (c.component_version || c.version || '-') + '</code></td>'
                            + '<td>' + (c.git_branch || c.branch || '<span style="color:#94a3b8">-</span>') + '</td>'
                            + '<td>' + (c.artifact_file || '<span style="color:#94a3b8">-</span>') + '</td></tr>';
                    });
                    html += '</tbody></table>';
                    body.innerHTML = html;
                } else {
                    body.innerHTML = '<div style="text-align:center;color:#94a3b8;padding:20px">暂无组件数据</div>';
                }
            })
            .catch(function(e) {
                body.innerHTML = '<div style="text-align:center;color:#ef4444">加载失败: ' + e.message + '</div>';
            });
    };

    // Check URL for saved parameter
    var params = new URLSearchParams(window.location.search);
    var saved = params.get('saved');
    if (saved) {
        var names = { jenkins: 'Jenkins', gitea: 'Gitea' };
        var displayName = names[saved] || saved;
        showToast('✅ ' + displayName + ' 配置保存成功', 3000);
        // Clean URL
        history.replaceState(null, '', window.location.pathname);
    }

    // Test API connectivity
    window.testAPI = function(btn, category) {
        var container = document.getElementById('apiStatus_' + category);
        var statusEl = container ? container.querySelector('.api-status') : null;
        var timeEl = container ? container.querySelector('.sys-test-time') : null;
        btn.disabled = true;
        btn.textContent = '测试中...';
        if (statusEl) { statusEl.className = 'api-status api-testing'; statusEl.textContent = '连接中...'; }

        // Step 1: Save current form values first
        var form = document.querySelector('.sys-config-form[data-category="' + category + '"]');
        var savePromise = Promise.resolve();
        if (form) {
            var formData = new URLSearchParams(new FormData(form));
            savePromise = fetch('/config/sys/' + category, {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: formData.toString()
            }).then(function(r) {
                if (!r.ok) { return r.text().then(function(t) { throw new Error(t || '保存失败'); }); }
            });
        }

        // Step 2: Test connection after save
        savePromise.then(function() {
            return fetch('/config/sys/' + category + '/test', { method: 'POST' });
        })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            var now = new Date();
            var timeStr = now.getFullYear() + '-' +
                String(now.getMonth()+1).padStart(2,'0') + '-' +
                String(now.getDate()).padStart(2,'0') + ' ' +
                String(now.getHours()).padStart(2,'0') + ':' +
                String(now.getMinutes()).padStart(2,'0') + ':' +
                String(now.getSeconds()).padStart(2,'0');
            if (data.ok) {
                if (statusEl) {
                    statusEl.className = 'api-status api-ok';
                    statusEl.textContent = '✓ 连接正常';
                }
                if (container) { container.setAttribute('data-status', 'ok'); container.setAttribute('data-time', timeStr); }
                if (timeEl) { timeEl.textContent = '最近测试: ' + timeStr; } else if (container && !timeEl) { var s = document.createElement('span'); s.className = 'sys-test-time'; s.textContent = '最近测试: ' + timeStr; container.appendChild(s); }
                showToast('✅ ' + (category === 'jenkins' ? 'Jenkins' : 'Gitea') + ' 连接正常', 2500);
            } else {
                if (statusEl) {
                    statusEl.className = 'api-status api-err';
                    statusEl.textContent = '✗ 连接异常';
                }
                if (container) { container.setAttribute('data-status', 'err'); container.setAttribute('data-time', timeStr); }
                if (timeEl) { timeEl.textContent = '最近测试: ' + timeStr; } else if (container && !timeEl) { var s = document.createElement('span'); s.className = 'sys-test-time'; s.textContent = '最近测试: ' + timeStr; container.appendChild(s); }
                showToast('❌ ' + (data.error || '连接失败'), 4000);
            }
        })
        .catch(function(e) {
            if (statusEl) { statusEl.className = 'api-status api-err'; statusEl.textContent = '✗ 连接异常'; }
            showToast('❌ ' + e.message, 4000);
        })
        .finally(function() {
            btn.disabled = false;
            btn.textContent = '测试连接';
        });
    };

    // Refresh page if there are running builds
    function checkRunningBuilds() {
        var badges = document.querySelectorAll('.badge-running, .badge-pending');
        if (badges.length > 0) {
            setTimeout(function() {
                window.location.reload();
            }, 10000);
        }
    }

    // Config tree loader for build form
    function initConfigTree(productId) {
        var box = document.getElementById('configTreeBox');
        if (!box) return;
        var url = '/config/tree/json';
        if (productId) url += '?product_id=' + productId;
        fetch(url)
            .then(function(r) { return r.json(); })
            .then(function(tree) {
                if (!tree || tree.length === 0) {
                    box.innerHTML = '<div class="config-empty-msg">暂无配置，请先在「配置管理」中添加组件和模块</div>';
                    return;
                }
                var html = '';
                tree.forEach(function(comp) {
                    var mods = comp.children || [];
                    var gitUrl = comp.git_url || '';
                    var branchFilter = comp.branch_filter || '';
                    html += '<div class="config-comp" data-comp-id="' + comp.id + '" data-comp-code="' + comp.code + '" data-git-url="' + gitUrl + '" data-branch-filter="' + branchFilter + '">';
                    html += '<label class="config-comp-header">';
                    html += '<input type="checkbox" class="comp-check" value="' + comp.id + '"> ';
                    html += '<span>' + comp.name + '</span>';
                    html += '<span class="config-comp-code">(' + comp.code + ')</span>';
                    if (mods.length > 0) html += ' <span style="font-size:0.75rem;color:#94a3b8">- ' + mods.length + '个模块</span>';
                    if (mods.length > 0) html += ' <a href="javascript:void(0)" class="mod-select-all" onclick="toggleAllMods(this)">全选</a>';
                    html += '</label>';
                    // Branch selector (hidden until component is checked)
                    html += '<div class="config-branch-select" style="display:none">';
                    html += '<label>分支: </label>';
                    html += '<select class="branch-select" name="comp_branch_' + comp.id + '"><option value="">加载中...</option></select>';
                    html += '</div>';
                    if (mods.length > 0) {
                        html += '<div class="config-modules">';
                        mods.forEach(function(mod) {
                            html += '<label class="config-mod-chip">';
                            html += '<input type="checkbox" class="mod-check" value="' + mod.id + '" data-code="' + mod.code + '"> ';
                            html += mod.name;
                            html += '</label>';
                        });
                        html += '</div>';
                    }
                    html += '</div>';
                });
                box.innerHTML = html;

                // Toggle modules visibility and branch loading when component is checked
                box.querySelectorAll('.comp-check').forEach(function(cb) {
                    cb.addEventListener('change', function() {
                        var comp = cb.closest('.config-comp');
                        var mods = comp.querySelector('.config-modules');
                        var branchDiv = comp.querySelector('.config-branch-select');
                        if (mods) {
                            if (cb.checked) mods.classList.add('open');
                            else mods.classList.remove('open');
                        }
                        // Show/hide branch selector and load branches
                        if (branchDiv) {
                            branchDiv.style.display = cb.checked ? '' : 'none';
                            if (cb.checked) loadBranches(comp);
                        }
                        updateComponentMods();
                    });
                });
                box.querySelectorAll('.mod-check').forEach(function(cb) {
                    cb.addEventListener('change', function() {
                        cb.closest('.config-mod-chip').classList.toggle('selected', cb.checked);
                        // Sync "select all" link text
                        var comp = cb.closest('.config-comp');
                        var link = comp.querySelector('.mod-select-all');
                        if (link) {
                            var all = comp.querySelectorAll('.mod-check');
                            var allChecked = all.length > 0;
                            all.forEach(function(c) { if (!c.checked) allChecked = false; });
                            link.textContent = allChecked ? '取消全选' : '全选';
                        }
                        updateComponentMods();
                    });
                });
                // Update hidden field when branch selection changes
                box.querySelectorAll('.branch-select').forEach(function(sel) {
                    sel.addEventListener('change', function() {
                        updateComponentMods();
                    });
                });
            })
            .catch(function() {
                box.innerHTML = '<div class="config-empty-msg">加载配置失败</div>';
            });
    }

    // Load branches for a component from Gitea API
    function loadBranches(compDiv) {
        var gitUrl = compDiv.getAttribute('data-git-url');
        var branchFilter = compDiv.getAttribute('data-branch-filter');
        var select = compDiv.querySelector('.branch-select');
        if (!select || !gitUrl) {
            if (select) select.innerHTML = '<option value="">未配置Git仓库</option>';
            return;
        }
        select.innerHTML = '<option value="">加载中...</option>';
        fetch('/api/gitea/branches?git_url=' + encodeURIComponent(gitUrl))
            .then(function(r) { return r.json(); })
            .then(function(branches) {
                if (!branches || branches.length === 0) {
                    select.innerHTML = '<option value="">无分支</option>';
                    return;
                }
                var html = '';
                var filter = branchFilter;
                branches.forEach(function(b) {
                    // Apply branch filter if configured
                    if (filter && filter.trim() && !matchBranch(b, filter)) return;
                    var selected = (filter && b === filter) ? ' selected' : '';
                    html += '<option value="' + b + '"' + selected + '>' + b + '</option>';
                });
                if (!html) html = '<option value="">无匹配分支</option>';
                select.innerHTML = html;
            })
            .catch(function() {
                select.innerHTML = '<option value="">加载失败</option>';
                // Remove any existing manual input first
                var existing = select.parentElement.querySelector('.branch-manual-input');
                if (existing) existing.remove();
                var input = document.createElement('input');
                input.type = 'text';
                input.className = 'branch-select branch-manual-input';
                input.placeholder = '\u8F93\u5165\u5206\u652F\u540D';
                input.name = select.name;
                select.removeAttribute('name'); // prevent select from being submitted
                select.parentElement.insertBefore(input, select.nextSibling);
            });
    }

    function matchBranch(branchName, filter) {
        if (!filter || filter.trim() === '') return true;
        // Support comma-separated patterns and wildcard *
        var patterns = filter.split(',');
        for (var i = 0; i < patterns.length; i++) {
            var p = patterns[i].trim();
            if (p === '') continue;
            if (p === '*') return true;
            if (p.indexOf('*') >= 0) {
                var re = new RegExp('^' + p.replace(/\*/g, '.*') + '$');
                if (re.test(branchName)) return true;
            } else if (branchName === p) {
                return true;
            }
        }
        return false;
    }

    function updateComponentMods() {
        var box = document.getElementById('configTreeBox');
        var hidden = document.getElementById('componentMods');
        if (!box || !hidden) return;
        var result = [];
        box.querySelectorAll('.config-comp').forEach(function(comp) {
            var cb = comp.querySelector('.comp-check');
            if (!cb.checked) return;
            var modules = [];
            comp.querySelectorAll('.mod-check:checked').forEach(function(mcb) {
                modules.push({ id: parseInt(mcb.value), code: mcb.getAttribute('data-code') });
            });
            var branchSelect = comp.querySelector('.branch-select');
            var branch = branchSelect ? branchSelect.value : '';
            result.push({
                component_id: parseInt(cb.value),
                component_code: comp.getAttribute('data-comp-code'),
                branch: branch,
                modules: modules
            });
        });
        hidden.value = JSON.stringify(result);
    }

    // Expandable table rows
    function initExpandRows() {
        document.querySelectorAll('.expand-btn').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                e.stopPropagation();
                var row = btn.closest('.row-main');
                var pid = row.getAttribute('data-product-id');
                var expandRow = document.querySelector('.row-expand[data-product-id="' + pid + '"]');
                if (!expandRow) return;

                var isOpen = expandRow.style.display !== 'none';
                if (isOpen) {
                    expandRow.style.display = 'none';
                    btn.classList.remove('active');
                    return;
                }

                // Open
                expandRow.style.display = '';
                btn.classList.add('active');

                // Only fetch once
                if (expandRow.getAttribute('data-loaded')) return;
                expandRow.setAttribute('data-loaded', '1');

                var loading = expandRow.querySelector('.expand-loading');
                var empty = expandRow.querySelector('.expand-empty');
                var table = expandRow.querySelector('.table-nested');
                loading.style.display = 'flex';

                fetch('/products/' + pid + '/releases/json')
                    .then(function(r) { return r.json(); })
                    .then(function(releases) {
                        loading.style.display = 'none';
                        if (!releases || releases.length === 0) {
                            empty.style.display = '';
                            return;
                        }
                        var tbody = table.querySelector('tbody');
                        releases.forEach(function(rel) {
                            var tr = document.createElement('tr');
                            tr.setAttribute('data-release-id', rel.id);
                            var date = rel.created_at ? new Date(rel.created_at).toLocaleString('zh-CN', {year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}) : '-';
                            var desc = rel.description || '';
                            var descPlain = stripHtml(desc);
                            var descShort = descPlain.length > 6 ? '<span class="badge-desc" onclick="showDescription(this)" data-full="' + escapeAttr(desc) + '" title="' + escapeAttr(descPlain) + '">' + descPlain.substring(0,6) + '...</span>' : (descPlain ? '<span class="badge-desc" onclick="showDescription(this)" data-full="' + escapeAttr(desc) + '" title="' + escapeAttr(descPlain) + '">' + descPlain + '</span>' : '<span style="color:#94a3b8">-</span>');
                            var compCount = rel.components ? rel.components.length : 0;
                            var compCell = compCount > 0 ? '<a href="javascript:void(0)" class="comp-count-link" onclick="showReleaseComps(this)" data-release-id="' + rel.id + '">' + compCount + ' 个组件</a>' : '<span style="color:#94a3b8">0</span>';
                            var bt = rel.build_types || {};
                            var actions = '<div style="white-space:nowrap">';
                            actions += '<button class="btn btn-sm" onclick="showManifest(' + rel.id + ')">查看 Manifest</button> ';
                            if (bt.upgrade) {
                                actions += '<a href="/releases/' + rel.id + '/download/upgrade" class="btn btn-sm btn-download" title="下载升级包">⬇ 升级包</a> ';
                            } else if (bt.full) {
                                actions += '<a href="/releases/' + rel.id + '/download/full" class="btn btn-sm btn-download" title="下载整包">⬇ 整包</a> ';
                            } else {
                                actions += '<button class="btn btn-sm" disabled style="opacity:0.4;cursor:not-allowed" title="无可用制品">⬇ 下载</button> ';
                            }
                            actions += '</div>';
                            tr.innerHTML =
                                '<td><strong>' + (rel.version||'') + '</strong></td>' +
                                '<td>' + descShort + '</td>' +
                                '<td>' + compCell + '</td>' +
                                '<td>' + date + '</td>' +
                                '<td>' + actions + '</td>';
                            tbody.appendChild(tr);
                        });
                        table.style.display = '';
                    })
                    .catch(function() {
                        loading.style.display = 'none';
                        empty.textContent = '加载失败';
                        empty.style.display = '';
                    });
            });
        });
    }

    // Component selection toggle (product form)
    window.toggleCompConfig = function(checkbox, compId) {
        var configDiv = document.getElementById('compConfig_' + compId);
        if (configDiv) {
            configDiv.style.display = checkbox.checked ? '' : 'none';
            if (checkbox.checked) {
                // Load Jenkins jobs for component select if in component mode
                var mode = document.querySelector('input[name="jenkins_job_mode"]:checked');
                if (mode && mode.value === 'component') {
                    var select = configDiv.querySelector('.jenkins-job-select');
                    if (select) loadJenkinsJobsForSelect(select);
                }
            }
        }
    };

    // Jenkins jobs cache
    var jenkinsJobsCache = null;
    var jenkinsJobsPromise = null;

    function loadJenkinsJobsForSelect(selectEl) {
        if (!selectEl || selectEl.getAttribute('data-loaded') === 'true') return;
        selectEl.setAttribute('data-loaded', 'true');

        if (jenkinsJobsCache) {
            populateJobSelect(selectEl, jenkinsJobsCache);
            return;
        }
        if (!jenkinsJobsPromise) {
            jenkinsJobsPromise = fetch('/api/jenkins/jobs')
                .then(function(r) { return r.json(); })
                .then(function(d) {
                    jenkinsJobsCache = d.jobs || [];
                    return jenkinsJobsCache;
                })
                .catch(function() {
                    jenkinsJobsCache = [];
                    return [];
                });
        }
        jenkinsJobsPromise.then(function(jobs) {
            populateJobSelect(selectEl, jobs);
        });
    }

    function populateJobSelect(selectEl, jobs) {
        var selected = selectEl.getAttribute('data-selected') || '';
        var html = '<option value="">不绑定</option>';
        jobs.forEach(function(job) {
            var sel = (job === selected) ? ' selected' : '';
            html += '<option value="' + job + '"' + sel + '>' + job + '</option>';
        });
        selectEl.innerHTML = html;
    }

    // On page load: populate Jenkins jobs for already-checked components (edit mode)
    document.addEventListener('DOMContentLoaded', function() {
        // Load jobs for project-level select
        var projectSelect = document.querySelector('.jenkins-job-select-project');
        if (projectSelect) {
            loadJenkinsJobsForProjectSelect(projectSelect);
        }
        // Load jobs for component-level selects if in component mode
        var mode = document.querySelector('input[name="jenkins_job_mode"]:checked');
        if (mode && mode.value === 'component') {
            document.querySelectorAll('.jenkins-job-select').forEach(function(select) {
                var configDiv = select.closest('.comp-config-fields');
                if (configDiv && configDiv.style.display !== 'none') {
                    loadJenkinsJobsForSelect(select);
                }
            });
        }
    });

    // Switch Jenkins binding mode
    window.switchJenkinsMode = function(mode) {
        var projectFields = document.getElementById('projectJobFields');
        if (projectFields) projectFields.style.display = (mode === 'project') ? '' : 'none';
        // Toggle per-component Jenkins fields
        document.querySelectorAll('.comp-jenkins-field').forEach(function(el) {
            el.style.display = (mode === 'component') ? '' : 'none';
        });
        if (mode === 'project') {
            var projectSelect = document.querySelector('.jenkins-job-select-project');
            if (projectSelect) loadJenkinsJobsForProjectSelect(projectSelect);
        }
        // Load jobs for visible component selects when switching to component mode
        if (mode === 'component') {
            document.querySelectorAll('.jenkins-job-select').forEach(function(select) {
                var field = select.closest('.comp-jenkins-field');
                if (field && field.style.display !== 'none') {
                    loadJenkinsJobsForSelect(select);
                }
            });
        }
    };

    // Load Jenkins jobs for project-level select
    function loadJenkinsJobsForProjectSelect(selectEl) {
        if (!selectEl || selectEl.getAttribute('data-loaded') === 'true') return;
        selectEl.setAttribute('data-loaded', 'true');

        if (jenkinsJobsCache) {
            populateProjectJobSelect(selectEl, jenkinsJobsCache);
            return;
        }
        if (!jenkinsJobsPromise) {
            jenkinsJobsPromise = fetch('/api/jenkins/jobs')
                .then(function(r) { return r.json(); })
                .then(function(d) {
                    jenkinsJobsCache = d.jobs || [];
                    return jenkinsJobsCache;
                })
                .catch(function() {
                    jenkinsJobsCache = [];
                    return [];
                });
        }
        jenkinsJobsPromise.then(function(jobs) {
            populateProjectJobSelect(selectEl, jobs);
        });
    }

    function populateProjectJobSelect(selectEl, jobs) {
        var selected = selectEl.getAttribute('data-selected') || '';
        var html = '<option value="">请选择 Jenkins 任务</option>';
        jobs.forEach(function(job) {
            var sel = (job === selected) ? ' selected' : '';
            html += '<option value="' + job + '"' + sel + '>' + job + '</option>';
        });
        selectEl.innerHTML = html;
    }
    // Toggle all modules for a component (build modal)
    window.toggleAllMods = function(link) {
        var comp = link.closest('.config-comp');
        var checks = comp.querySelectorAll('.mod-check');
        var allChecked = true;
        checks.forEach(function(cb) { if (!cb.checked) allChecked = false; });
        var newVal = !allChecked;
        checks.forEach(function(cb) {
            cb.checked = newVal;
            cb.closest('.config-mod-chip').classList.toggle('selected', newVal);
        });
        link.textContent = newVal ? '取消全选' : '全选';
        updateComponentMods();
    };
    // Test environment toggle (product form)
    window.toggleTestEnv = function() {
        var checked = document.getElementById('testEnvEnabled').checked;
        document.getElementById('testEnvFields').style.display = checked ? '' : 'none';
    };

    // Product form validation
    window.validateProductForm = function(form) {
        var compChecks = form.querySelectorAll('input[name="comp_ids"]:checked');
        if (compChecks.length === 0) {
            alert('请至少选择一个组件');
            return false;
        }
        for (var i = 0; i < compChecks.length; i++) {
            var id = compChecks[i].value;
            var gitInput = form.querySelector('input[name="comp_git_url_' + id + '"]');
            if (!gitInput || !gitInput.value.trim()) {
                var label = compChecks[i].closest('label');
                var name = label ? label.querySelector('strong')?.textContent || '组件' : '组件';
                alert('组件「' + name + '」必须填写Git仓库地址');
                gitInput && gitInput.focus();
                return false;
            }
        }
        var testEnvCb = document.getElementById('testEnvEnabled');
        if (testEnvCb && testEnvCb.checked) {
            var envChecks = form.querySelectorAll('input[name="test_env_ids"]:checked');
            if (envChecks.length === 0) {
                alert('启用测试环境时，请至少选择一个测试环境');
                return false;
            }
        }
        return true;
    };

    // Repo search dropdown (product form)
    var repoCache = {};
    var repoTimer = null;

    function fetchRepos(query, callback) {
        clearTimeout(repoTimer);
        repoTimer = setTimeout(function() {
            var url = '/api/gitea/repos' + (query ? '?q=' + encodeURIComponent(query) : '');
            fetch(url)
                .then(function(r) { return r.json(); })
                .then(function(data) { callback(data); })
                .catch(function() { callback([]); });
        }, 300);
    }

    function renderDropdown(dropdown, repos, input, hiddenInput) {
        if (!repos || repos.length === 0) {
            dropdown.innerHTML = '<div class="repo-dropdown-empty">无匹配仓库</div>';
            dropdown.style.display = '';
            return;
        }
        var html = '';
        repos.forEach(function(repo) {
            html += '<div class="repo-dropdown-item" data-url="' + repo.clone_url + '" data-name="' + repo.full_name + '">'
                + '<span class="repo-name">' + repo.full_name + '</span>'
                + '</div>';
        });
        dropdown.innerHTML = html;
        dropdown.style.display = '';

        dropdown.querySelectorAll('.repo-dropdown-item').forEach(function(item) {
            item.addEventListener('mousedown', function(e) {
                e.preventDefault();
                input.value = item.getAttribute('data-name');
                hiddenInput.value = item.getAttribute('data-url');
                dropdown.style.display = 'none';
                input.classList.add('repo-selected');
            });
        });
    }

    document.querySelectorAll('.repo-search-input').forEach(function(input) {
        var wrapper = input.closest('.repo-select-wrapper');
        var dropdown = wrapper.querySelector('.repo-dropdown');
        var hiddenInput = wrapper.querySelector('.repo-url-value');

        input.addEventListener('focus', function() {
            if (input.value === '') {
                fetchRepos('', function(repos) { renderDropdown(dropdown, repos, input, hiddenInput); });
            }
        });

        input.addEventListener('input', function() {
            hiddenInput.value = '';
            input.classList.remove('repo-selected');
            var q = input.value.trim();
            fetchRepos(q, function(repos) { renderDropdown(dropdown, repos, input, hiddenInput); });
        });

        input.addEventListener('blur', function() {
            setTimeout(function() { dropdown.style.display = 'none'; }, 150);
        });
    });

    // Build modal
    window.openBuildModal = function(productId, productName, currentVersion) {
        var modal = document.getElementById('buildModal');
        var form = document.getElementById('buildForm');
        var title = document.getElementById('buildModalTitle');
        if (!modal) return;
        form.action = '/products/' + productId + '/build';
        title.textContent = '触发构建 - ' + productName;
        // Reset form state
        form.reset();
        document.getElementById('specifyVersion').checked = false;
        document.getElementById('isFormal').checked = false;
        document.getElementById('versionInputGroup').style.display = 'none';
        document.getElementById('releaseNotesGroup').style.display = 'none';
        // Reset Quill editor if exists
        if (window._quillEditor) {
            window._quillEditor.setText('');
        }
        document.getElementById('releaseNotesHidden').value = '';
        document.querySelector('input[name="auto_sync_test"]').checked = true;
        document.querySelector('input[name="build_type"][value="upgrade"]').checked = true;
        // Reset component selection visibility
        var configGroup = document.getElementById('configTreeGroup');
        if (configGroup) configGroup.style.display = '';
        // Pre-fill version as default value (hidden until checkbox is ticked)
        var versionInput = document.getElementById('buildVersion');
        versionInput.value = '';
        versionInput.disabled = true;
        modal.style.display = 'flex';
        // Load config tree
        initConfigTree(productId);
    };
    window.closeBuildModal = function() {
        var modal = document.getElementById('buildModal');
        if (modal) modal.style.display = 'none';
    };
    window.toggleVersionInput = function() {
        var checked = document.getElementById('specifyVersion').checked;
        var input = document.getElementById('buildVersion');
        document.getElementById('versionInputGroup').style.display = checked ? '' : 'none';
        if (!checked) {
            input.value = '';
            input.disabled = true;
        } else {
            input.disabled = false;
        }
    };
    window.toggleReleaseNotes = function() {
        var checked = document.getElementById('isFormal').checked;
        var group = document.getElementById('releaseNotesGroup');
        group.style.display = checked ? '' : 'none';
        if (checked && !window._quillEditor && typeof Quill !== 'undefined') {
            window._quillEditor = new Quill('#releaseNotesEditor', {
                theme: 'snow',
                placeholder: '变更说明...',
                modules: {
                    toolbar: [
                        ['bold', 'italic', 'underline'],
                        [{ 'list': 'ordered'}, { 'list': 'bullet' }],
                        ['clean']
                    ]
                }
            });
        }
    };
    // Toggle component selection visibility based on build type
    window.toggleBuildType = function() {
        var buildType = document.querySelector('input[name="build_type"]:checked').value;
        var group = document.getElementById('configTreeGroup');
        if (group) {
            group.style.display = (buildType === 'full') ? 'none' : '';
        }
    };
    // Close build modal on backdrop click
    (function() {
        var bm = document.getElementById('buildModal');
        if (bm) {
            bm.addEventListener('click', function(e) {
                if (e.target === bm) closeBuildModal();
            });
        }
    })();

    // Build form AJAX submit with confirmation
    (function() {
        var buildForm = document.getElementById('buildForm');
        if (!buildForm) return;
        buildForm.addEventListener('submit', function(e) {
            e.preventDefault();
            // Show confirmation dialog
            var modal = document.getElementById('confirmModal');
            var msgEl = document.getElementById('confirmModalMsg');
            var okBtn = document.getElementById('confirmOk');
            var cancelBtn = document.getElementById('confirmCancel');
            if (!modal) return;
            msgEl.textContent = '确认触发构建？';
            modal.style.display = 'flex';

            function closeConfirm() {
                modal.style.display = 'none';
                okBtn.replaceWith(okBtn.cloneNode(true));
                cancelBtn.replaceWith(cancelBtn.cloneNode(true));
            }

            document.getElementById('confirmCancel').onclick = closeConfirm;
            document.getElementById('confirmOk').onclick = function() {
                closeConfirm();
                // Sync Quill editor content to hidden input
                var hiddenInput = document.getElementById('releaseNotesHidden');
                if (window._quillEditor && hiddenInput) {
                    var html = window._quillEditor.root.innerHTML;
                    // Quill returns <p><br></p> for empty content
                    hiddenInput.value = (html === '<p><br></p>' || html === '') ? '' : html;
                }
                var submitBtn = buildForm.querySelector('button[type=submit]');
                var origText = submitBtn ? submitBtn.textContent : '';
                if (submitBtn) { submitBtn.disabled = true; submitBtn.textContent = '提交中...'; }
                var formData = new URLSearchParams(new FormData(buildForm));
                fetch(buildForm.action, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: formData.toString(),
                    redirect: 'manual'
                }).then(function(resp) {
                    if (submitBtn) { submitBtn.disabled = false; submitBtn.textContent = origText; }
                    closeBuildModal();
                    if (resp.type === 'opaqueredirect' || resp.ok) {
                        showToast('✅ 构建任务已发送', 3000);
                        // Refresh product detail page after build submission
                        if (/^\/products\/\d+$/.test(window.location.pathname)) {
                            setTimeout(function() { window.location.reload(); }, 1500);
                        }
                    } else {
                        showToast('❌ 触发构建失败', 4000);
                    }
                }).catch(function(err) {
                    if (submitBtn) { submitBtn.disabled = false; submitBtn.textContent = origText; }
                    showToast('❌ 触发构建失败: ' + err.message, 4000);
                });
            };
        });
    })();

    // Card collapse/expand
    window.toggleCollapse = function(header) {
        var card = header.closest('.card');
        if (!card) return;
        var id = card.getAttribute('data-collapse-id');
        var collapsed = card.classList.toggle('collapsed');
        var textEl = header.querySelector('.collapse-text');
        if (textEl) textEl.textContent = collapsed ? '展开' : '缩起';
        if (id) {
            try { localStorage.setItem('collapse_' + id, collapsed ? '1' : '0'); } catch(e) {}
        }
    };

    // Toggle all cards expand/collapse (single toggle button)
    window.toggleAllCards = function() {
        var cards = document.querySelectorAll('.card[data-collapse-id]');
        var allCollapsed = true;
        cards.forEach(function(card) { if (!card.classList.contains('collapsed')) allCollapsed = false; });
        var expand = allCollapsed;
        cards.forEach(function(card) {
            var id = card.getAttribute('data-collapse-id');
            var collapsed = !expand;
            card.classList.toggle('collapsed', collapsed);
            var textEl = card.querySelector('.collapse-text');
            if (textEl) textEl.textContent = collapsed ? '展开' : '缩起';
            if (id) {
                try { localStorage.setItem('collapse_' + id, collapsed ? '1' : '0'); } catch(e) {}
            }
        });
        var btn = document.getElementById('toggleAllBtn');
        if (btn) btn.textContent = expand ? '全部缩起' : '全部展开';
    };

    // AJAX form submit for config page (no page jump)
    function initAjaxForms() {
        document.addEventListener('submit', function(e) {
            var form = e.target;
            if (!form.hasAttribute('data-ajax-form')) return;
            if (form.getAttribute('data-confirm')) return;
            e.preventDefault();
            var btn = form.querySelector('button[type=submit]');
            var origText = btn ? btn.textContent : '';
            if (btn) { btn.disabled = true; btn.textContent = '保存中...'; }
            var formData = new URLSearchParams(new FormData(form));
            fetch(form.action, {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: formData.toString(),
                redirect: 'manual'
            }).then(function() {
                showToast('✅ 保存成功', 2000);
                sessionStorage.setItem('configScrollY', window.scrollY.toString());
                setTimeout(function() { window.location.href = '/config'; }, 600);
            }).catch(function(err) {
                if (btn) { btn.disabled = false; btn.textContent = origText; }
                showToast('❌ 保存失败: ' + err.message, 4000);
            });
        }, true);

        // Restore scroll position after AJAX reload
        var savedY = sessionStorage.getItem('configScrollY');
        if (savedY !== null && window.location.pathname === '/config') {
            sessionStorage.removeItem('configScrollY');
            var y = parseInt(savedY, 10) || 0;
            setTimeout(function() { window.scrollTo(0, y); }, 50);
        }
    }
    window.toggleAddForm = function(formId) {
        var form = document.getElementById(formId);
        if (!form) return;
        var isHidden = form.style.display === 'none';
        form.style.display = isHidden ? '' : 'none';
        if (isHidden) {
            var firstInput = form.querySelector('input:not([type=hidden])');
            if (firstInput) firstInput.focus();
        }
    };

    function initCollapse() {
        document.querySelectorAll('.card[data-collapse-id]').forEach(function(card) {
            var id = card.getAttribute('data-collapse-id');
            try {
                if (localStorage.getItem('collapse_' + id) === '1') {
                    card.classList.add('collapsed');
                    var textEl = card.querySelector('.collapse-text');
                    if (textEl) textEl.textContent = '展开';
                }
            } catch(e) {}
        });
    }

    // Inline edit toggle for config page
    window.toggleEdit = function(btn) {
        var tr = btn.closest('tr');
        var id = tr.getAttribute('data-id');
        // Find sibling rows with same data-id
        var viewRow = tr.closest('table').querySelector('tr.row-view[data-id="' + id + '"]');
        var editRow = tr.closest('table').querySelector('tr.row-edit[data-id="' + id + '"]');
        if (!viewRow || !editRow) return;
        var isEditing = editRow.style.display !== 'none';
        if (isEditing) {
            editRow.style.display = 'none';
            viewRow.style.display = '';
        } else {
            viewRow.style.display = 'none';
            editRow.style.display = '';
        }
    };

    // Confirm modal logic
    function initConfirmModal() {
        var modal = document.getElementById('confirmModal');
        var msgEl = document.getElementById('confirmModalMsg');
        var cancelBtn = document.getElementById('confirmCancel');
        var okBtn = document.getElementById('confirmOk');
        if (!modal) return;

        function closeModal() {
            modal.style.display = 'none';
            modal.removeAttribute('data-pending-action');
            modal.removeAttribute('data-pending-url');
        }

        cancelBtn.addEventListener('click', closeModal);
        modal.addEventListener('click', function(e) {
            if (e.target === modal) closeModal();
        });

        okBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            var actionType = modal.getAttribute('data-pending-action');
            var actionUrl = modal.getAttribute('data-pending-url');
            var isAjaxForm = modal.getAttribute('data-ajax-form') === 'true';
            closeModal();
            if (isAjaxForm && actionUrl) {
                fetch(actionUrl, { method: 'POST', redirect: 'manual' })
                    .then(function() {
                        showToast('✅ 操作成功', 2000);
                        sessionStorage.setItem('configScrollY', window.scrollY.toString());
                        setTimeout(function() { window.location.href = '/config'; }, 600);
                    })
                    .catch(function() {
                        showToast('✅ 操作成功', 2000);
                        sessionStorage.setItem('configScrollY', window.scrollY.toString());
                        setTimeout(function() { window.location.href = '/config'; }, 600);
                    });
            } else if (actionType === 'form' && actionUrl) {
                fetch(actionUrl, { method: 'POST', redirect: 'follow' })
                    .then(function() { window.location.reload(); })
                    .catch(function() { window.location.reload(); });
            }
        });

        // Intercept forms with data-confirm attribute
        document.addEventListener('submit', function(e) {
            var form = e.target;
            var msg = form.getAttribute('data-confirm');
            if (!msg) return;
            e.preventDefault();
            msgEl.textContent = msg;
            modal.setAttribute('data-pending-action', 'form');
            modal.setAttribute('data-pending-url', form.action);
            if (form.hasAttribute('data-ajax-form')) modal.setAttribute('data-ajax-form', 'true');
            else modal.removeAttribute('data-ajax-form');
            modal.style.display = 'flex';
        }, true);

        // Intercept links/buttons with data-confirm attribute (not inside a data-confirm form)
        document.addEventListener('click', function(e) {
            // Skip if click is inside a form that has data-confirm (the submit handler will handle it)
            if (e.target.closest('form[data-confirm]')) return;
            var el = e.target.closest('[data-confirm]');
            if (!el) return;
            var msg = el.getAttribute('data-confirm');
            if (!msg) return;
            e.preventDefault();
            e.stopPropagation();
            msgEl.textContent = msg;
            modal.setAttribute('data-pending-action', 'click');
            modal.style.display = 'flex';
        }, true);
    }

    // ========================================
    // Script Management Modal (Playwright)
    // ========================================
    var smEnvId = null;
    var smData = {};

    // Helper: apply Prism.js highlighting to the script preview
    function highlightPreview(code) {
        var el = document.getElementById('smPreviewCode');
        if (!el) return;
        el.textContent = code || '暂无脚本内容';
        if (code && typeof Prism !== 'undefined') {
            el.className = 'script-preview-code language-javascript';
            Prism.highlightElement(el);
        }
    }

    // Load script data from embedded JSON
    (function() {
        var el = document.getElementById('scriptData');
        if (el) { try { smData = JSON.parse(el.textContent); } catch(e) {} }
    })();

    window.openScriptModal = function(envId, envName) {
        smEnvId = envId;
        var d = smData[envId] || {};
        document.getElementById('smEnvName').textContent = envName;
        // status
        var labels = { idle: '未执行', running: '执行中...', success: '执行成功', failed: '执行失败' };
        var badge = document.getElementById('smStatus');
        badge.className = 'script-status-badge script-status-' + (d.status || 'idle');
        badge.textContent = labels[d.status] || '未执行';
        document.getElementById('smTime').textContent = d.time || '';
        // preview
        var code = d.content || '';
        highlightPreview(code);
        document.getElementById('smPreview').style.display = '';
        document.getElementById('smEditor').style.display = 'none';
        document.getElementById('smRecording').style.display = 'none';
        document.getElementById('smTextarea').value = code;
        // log
        if (d.output) {
            document.getElementById('smLog').textContent = d.output;
            document.getElementById('smLogPanel').style.display = '';
        } else {
            document.getElementById('smLogPanel').style.display = 'none';
        }
        document.getElementById('scriptModal').style.display = 'flex';
    };

    window.closeScriptModal = function() {
        document.getElementById('scriptModal').style.display = 'none';
        smEnvId = null;
    };

    window.startRecording = function() {
        if (!smEnvId) return;
        var modal = document.getElementById('recordingConfirmModal');
        if (!modal) {
            modal = document.createElement('div');
            modal.id = 'recordingConfirmModal';
            modal.className = 'modal-overlay';
            modal.style.zIndex = '10000';
            modal.innerHTML = '<div class="modal-box" style="width:580px;max-width:90vw;text-align:left">' +
                '<div class="modal-header"><h3 style="margin:0;font-size:1.05rem">🎬 \u5F55\u5236\u786E\u8BA4</h3>' +
                '<button class="btn-close-modal" onclick="document.getElementById(\'recordingConfirmModal\').style.display=\'none\'">&times;</button></div>' +
                '<div style="padding:20px 24px" id="recordingConfirmMsg"></div>' +
                '<div style="text-align:right;padding:14px 24px;border-top:1px solid #f1f5f9;background:#f8fafc;border-radius:0 0 12px 12px">' +
                '<button class="btn" onclick="document.getElementById(\'recordingConfirmModal\').style.display=\'none\'">\u53D6\u6D88</button>' +
                '<button class="btn btn-primary" style="margin-left:8px" onclick="confirmStartRecording()">\u5F00\u59CB\u5F55\u5236</button></div></div>';
            document.body.appendChild(modal);
        }
        var msg = document.getElementById('recordingConfirmMsg');
        msg.innerHTML = '<p style="margin:0 0 12px;color:#334155">\u5373\u5C06\u542F\u52A8 Playwright \u5F55\u5236\u6D4F\u89C8\u5668\uFF0C\u8BF7\u6309\u4EE5\u4E0B\u6B65\u9AA4\u64CD\u4F5C\uFF1A</p>' +
            '<ol style="margin:0;padding-left:20px;color:#475569;line-height:1.8">' +
            '<li>\u5728\u6D4F\u89C8\u5668\u4E2D\u5B8C\u6210\u5347\u7EA7\u64CD\u4F5C</li>' +
            '<li>\u5728 Playwright Inspector \u7A97\u53E3\u70B9\u51FB\u300CRecord\u300D\u6309\u94AE\u505C\u6B62\u5F55\u5236</li>' +
            '<li>\u5173\u95ED\u6D4F\u89C8\u5668\u7A97\u53E3</li></ol>' +
            '<div style="margin-top:14px;padding:10px 14px;background:#fef2f2;border-radius:6px;color:#dc2626;font-size:0.85rem">' +
            '⚠ \u5FC5\u987B\u5148\u70B9\u51FB\u300CRecord\u300D\u6309\u94AE\u505C\u6B62\u5F55\u5236\uFF0C\u5426\u5219\u4E0D\u4F1A\u751F\u6210\u811A\u672C\u6587\u4EF6</div>';
        modal.style.display = 'flex';
    };

    window.confirmStartRecording = function() {
        var modal = document.getElementById('recordingConfirmModal');
        if (modal) modal.style.display = 'none';
        document.getElementById('smPreview').style.display = 'none';
        document.getElementById('smEditor').style.display = 'none';
        document.getElementById('smRecording').style.display = 'flex';

        fetch('/config/testenv/' + smEnvId + '/script/record', { method: 'POST' })
            .then(function(r) { if (!r.ok) return r.text().then(function(t) { throw new Error(t); }); return r.json(); })
            .then(function() { pollRecording(); })
            .catch(function(err) {
                document.getElementById('smRecording').style.display = 'none';
                document.getElementById('smPreview').style.display = '';
                showToast('录制失败: ' + err.message, 4000);
            });
    };

    function pollRecording() {
        var iv = setInterval(function() {
            fetch('/config/testenv/' + smEnvId + '/script/record/status')
                .then(function(r) { return r.json(); })
                .then(function(d) {
                    if (!d.done) return;
                    clearInterval(iv);
                    document.getElementById('smRecording').style.display = 'none';
                    document.getElementById('smPreview').style.display = '';
                    if (d.error) {
                        showToast('录制失败: ' + d.error, 4000);
                    } else {
                        var code = d.content || '';
                        highlightPreview(code);
                        document.getElementById('smTextarea').value = code;
                        if (smData[smEnvId]) smData[smEnvId].content = code;
                        showToast('录制完成，脚本已保存', 3000);
                    }
                })
                .catch(function(err) { clearInterval(iv); showToast('轮询失败: ' + err.message, 4000); });
        }, 2000);
    }

    window.toggleSmEdit = function() {
        var editor = document.getElementById('smEditor');
        var preview = document.getElementById('smPreview');
        var isEdit = editor.style.display !== 'none';
        editor.style.display = isEdit ? 'none' : 'block';
        preview.style.display = isEdit ? '' : 'none';
        if (!isEdit) document.getElementById('smTextarea').value = (smData[smEnvId] || {}).content || '';
    };

    window.saveScript = function() {
        if (!smEnvId) return;
        var content = document.getElementById('smTextarea').value;
        var params = new URLSearchParams();
        params.set('script_content', content);
        fetch('/config/testenv/' + smEnvId + '/script/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: params.toString()
        })
        .then(function(r) { return r.json(); })
        .then(function(d) {
            if (d.error) { showToast(d.error, 4000); return; }
            if (!smData[smEnvId]) smData[smEnvId] = {};
            smData[smEnvId].content = content;
            highlightPreview(content);
            toggleSmEdit();
            showToast('保存成功', 2000);
        })
        .catch(function(err) { showToast('保存失败: ' + err.message, 4000); });
    };

    // Insert file upload operation into script textarea
    window.insertFileOp = function() {
        var filename = prompt('请输入制品文件名（例如：acis_main_3.1.2_upgrade.zip）：', '');
        if (!filename) return;
        var snippet = "await page.locator('iframe').contentFrame().locator('input[type=\"file\"]').setInputFiles('" + filename + "');";
        var ta = document.getElementById('smTextarea');
        if (!ta) return;
        var start = ta.selectionStart;
        var end = ta.selectionEnd;
        var before = ta.value.substring(0, start);
        var after = ta.value.substring(end);
        // Insert with proper indentation (newline before if not at start)
        var insert = snippet;
        if (before.length > 0 && !before.endsWith('\n')) insert = '\n' + insert;
        if (after.length > 0 && !after.startsWith('\n')) insert = insert + '\n';
        ta.value = before + insert + after;
        ta.selectionStart = ta.selectionEnd = start + insert.length;
        ta.focus();
        showToast('已插入文件上传操作', 2000);
    };

    window.runScriptExec = function() {
        if (!smEnvId) return;
        var btn = document.querySelector('.script-modal-actions .btn-run');
        btn.disabled = true; btn.textContent = '执行中...';
        var badge = document.getElementById('smStatus');
        badge.className = 'script-status-badge script-status-running';
        badge.textContent = '执行中...';
        document.getElementById('smLog').textContent = '正在执行脚本...';
        document.getElementById('smLogPanel').style.display = '';

        fetch('/config/testenv/' + smEnvId + '/script/run', { method: 'POST' })
            .then(function(r) { return r.json(); })
            .then(function() { pollExecStatus(); })
            .catch(function(err) {
                btn.disabled = false; btn.textContent = '执行';
                showToast('启动失败: ' + err.message, 4000);
            });
    };

    function pollExecStatus() {
        var iv = setInterval(function() {
            fetch('/config/testenv/' + smEnvId + '/script/output')
                .then(function(r) { return r.json(); })
                .then(function(d) {
                    document.getElementById('smLog').textContent = d.output || '(无输出)';
                    if (d.status !== 'running') {
                        clearInterval(iv);
                        var btn = document.querySelector('.script-modal-actions .btn-run');
                        btn.disabled = false; btn.textContent = '执行';
                        var labels = { idle: '未执行', success: '执行成功', failed: '执行失败' };
                        var badge = document.getElementById('smStatus');
                        badge.className = 'script-status-badge script-status-' + d.status;
                        badge.textContent = labels[d.status] || d.status;
                        if (smData[smEnvId]) {
                            smData[smEnvId].status = d.status;
                            smData[smEnvId].output = d.output;
                            smData[smEnvId].time = d.last_run || '';
                        }
                        document.getElementById('smTime').textContent = d.last_run || '';
                    }
                })
                .catch(function(err) { clearInterval(iv); showToast('轮询失败: ' + err.message, 4000); });
        }, 2000);
    }

    // Build history script log toggle
    window.showScriptLog = function(buildId) {
        var row = document.getElementById('script-log-' + buildId);
        if (!row) return;
        row.style.display = row.style.display === 'none' ? 'table-row' : 'none';
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            checkRunningBuilds();
            initConfirmModal();
            initAjaxForms();
            initExpandRows();
            initConfigTree();
            initCollapse();
        });
    } else {
        checkRunningBuilds();
        initConfirmModal();
        initAjaxForms();
        initExpandRows();
        initConfigTree();
        initCollapse();
    }

})();
