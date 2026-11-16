/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
const DASHBOARD_EXCLUDE_ADMINS_STORAGE_KEY = 'dashboard_exclude_admins'

export function getSavedExcludeAdmins(): boolean {
  if (typeof window === 'undefined') return false
  return localStorage.getItem(DASHBOARD_EXCLUDE_ADMINS_STORAGE_KEY) === '1'
}

export function saveExcludeAdmins(value: boolean): void {
  if (typeof window === 'undefined') return
  localStorage.setItem(DASHBOARD_EXCLUDE_ADMINS_STORAGE_KEY, value ? '1' : '0')
}
