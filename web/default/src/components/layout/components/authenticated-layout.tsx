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
import { getCookie } from '@/lib/cookies'
import { cn } from '@/lib/utils'
import { LayoutProvider } from '@/context/layout-provider'
import { SearchProvider } from '@/context/search-provider'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { AnimatedOutlet } from '@/components/page-transition'
import { SkipToMain } from '@/components/skip-to-main'
import { WorkspaceProvider } from '../context/workspace-context'
import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout(props: AuthenticatedLayoutProps) {
  const defaultOpen = getCookie('sidebar_state') !== 'false'

  return (
    <LayoutProvider>
      <SearchProvider>
        <WorkspaceProvider>
          <div style={{ '--app-header-height': '0px' } as React.CSSProperties}>
            <SidebarProvider defaultOpen={defaultOpen}>
              <SkipToMain />
              <div className='flex min-h-svh w-full flex-1'>
                <AppSidebar />
                <SidebarInset
                  className={cn(
                    '@container/content',
                    'flex flex-col flex-1 min-w-0 bg-background'
                  )}
                >
                  <div style={{ '--app-header-height': '3.5rem' } as React.CSSProperties}>
                    <AppHeader />
                  </div>
                  <div className='flex-1 overflow-auto'>
                    {props.children ?? <AnimatedOutlet />}
                  </div>
                </SidebarInset>
              </div>
            </SidebarProvider>
          </div>
        </WorkspaceProvider>
      </SearchProvider>
    </LayoutProvider>
  )
}
