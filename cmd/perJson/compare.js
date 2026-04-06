const fs = require('fs');

// Đọc 2 file
const index1 = JSON.parse(fs.readFileSync('index1.json', 'utf8'));
const index = JSON.parse(fs.readFileSync('index.json', 'utf8'));

// Lấy danh sách handles
const handles1 = new Set(index1.permissions.map(p => p.handle));
const handles = new Set(index.permissions.map(p => p.handle));

// Tìm những quyền trong index.json mà KHÔNG có trong index1.json
const missing = [];
index.permissions.forEach(p => {
  if (!handles1.has(p.handle)) {
    missing.push(p);
  }
});

console.log('=== Những quyền trong index.json mà index1.json THIẾU ===\n');
console.log('Total thiếu:', missing.length);
console.log();

missing.forEach((p, i) => {
  console.log(`${i + 1}. ${p.handle}`);
  console.log(`   ${p.description}`);
  console.log();
});

// Xuất ra file JSON
const result = {
  "total_missing": missing.length,
  "missing_permissions": missing
};

fs.writeFileSync('missing_permissions.json', JSON.stringify(result, null, 2));
console.log('Đã lưu kết quả vào missing_permissions.json');
