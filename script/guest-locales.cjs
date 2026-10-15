// Shared by both compilers. Only display vocabulary is exported to viewers.
const blocked = /login|log_in|logout|password|account|oauth|oidc|sso|session|admin|onboarding|terminal|file_manager|uninstall|recovery|migration/i;
function filter(value) {
  if (Array.isArray(value)) return value.map(filter);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value)
    .filter(([key]) => !blocked.test(key))
    .map(([key, child]) => [key, filter(child)])
    .filter(([, child]) => child !== undefined));
  if (typeof value === 'string' && /log[ -]?in|sign[ -]?in|log[ -]?out|password|\/admin\b|\/api\/me\b|登录|登陆|登入|登出|密码|密碼|アカウント|ログイン|パスワード|masuk|kata sandi/i.test(value)) return undefined;
  return value;
}
module.exports = filter;
