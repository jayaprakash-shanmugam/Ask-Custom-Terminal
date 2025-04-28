import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AuditDetailComponent } from './audit-detail/audit-detail.component';
import { AuditListComponent } from './audit-list/audit-list.component';
import { DashboardComponent } from './dashboard/dashboard.component';
import { SessionViewComponent } from './session-view/session-view.component';
import { StatsComponent } from './stats/stats.component';

const routes: Routes = [
  { path: '', redirectTo: '/dashboard', pathMatch: 'full' },
  { path: 'dashboard', component: DashboardComponent },
  { path: 'clients', component: AuditListComponent, data: { type: 'client' } },
  { path: 'models', component: AuditListComponent, data: { type: 'model' } },
  { path: 'client/:id', component: AuditDetailComponent, data: { type: 'client' } },
  { path: 'model/:id', component: AuditDetailComponent, data: { type: 'model' } },
  { path: 'session/:sessionId', component: SessionViewComponent },
  { path: 'stats', component: StatsComponent },
  { path: '', redirectTo: '/dashboard', pathMatch: 'full' }, // Default route
];

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule]
})
export class AppRoutingModule { }