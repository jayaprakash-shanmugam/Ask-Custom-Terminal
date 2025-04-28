import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { forkJoin } from 'rxjs';
import { AuditEvent } from '../models/audit-event';
import { AuditService } from '../services/audit.service';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.css']
})
export class DashboardComponent implements OnInit {
  recentClientAudits: AuditEvent[] = [];
  recentModelAudits: AuditEvent[] = [];
  statsLoaded = false;
  clientTotal = 0;
  modelTotal = 0;
  loading = true;
  error: string | null = null;

  constructor(
    private auditService: AuditService,
    private router: Router
  ) {}

  ngOnInit(): void {
    this.loadDashboardData();
  }

  loadDashboardData(): void {
    this.loading = true;
    this.error = null;

    forkJoin({
      clientAudits: this.auditService.getClientAudits(1, 5),
      modelAudits: this.auditService.getModelAudits(1, 5),
      stats: this.auditService.getAuditStats()
    }).subscribe({
      next: (results) => {
        this.recentClientAudits = results.clientAudits.data;
        this.recentModelAudits = results.modelAudits.data;
        this.clientTotal = results.stats.clientTotal;
        this.modelTotal = results.stats.modelTotal;
        this.statsLoaded = true;
        this.loading = false;
      },
      error: (err) => {
        this.error = 'Failed to load dashboard data. Please try again later.';
        this.loading = false;
        console.error('Dashboard data loading error:', err);
      }
    });
  }

  viewAuditDetails(audit: AuditEvent, type: 'client' | 'model'): void {
    this.router.navigate([`/${type}/${audit._id}`]);
  }

  viewAllClientAudits(): void {
    this.router.navigate(['/clients']);
  }

  viewAllModelAudits(): void {
    this.router.navigate(['/models']);
  }

  viewStats(): void {
    this.router.navigate(['/stats']);
  }

  formatTimestamp(timestamp: Date): string {
    return new Date(timestamp).toLocaleString();
  }

  viewSessionAudits(sessionId: string): void {
    if (sessionId) {
      this.router.navigate(['/session', sessionId]);
    }
  }
}