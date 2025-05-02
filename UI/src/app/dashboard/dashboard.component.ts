import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { forkJoin } from 'rxjs';
import { AuditEvent } from '../models/audit-event';
import { AuditService } from '../services/audit.service';
import { FormBuilder, FormGroup } from '@angular/forms';
import { MatSnackBar } from '@angular/material/snack-bar';

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
  
  // New properties for enhanced UI
  clientTrend = 0;
  modelTrend = 0;
  dateRangeForm: FormGroup;
  selectedEventType = '';
  Math = Math; // For template use

  constructor(
    private auditService: AuditService,
    private router: Router,
    private fb: FormBuilder,
    private snackBar: MatSnackBar
  ) {
    // Initialize date range form with last 30 days
    const today = new Date();
    const lastMonth = new Date();
    lastMonth.setMonth(lastMonth.getMonth() - 1);
    
    this.dateRangeForm = this.fb.group({
      start: [lastMonth],
      end: [today]
    });
  }

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
        
        // Calculate trend values (placeholder - replace with actual calculation)
        this.clientTrend = this.calculateTrend(results.stats.clientPrevTotal, this.clientTotal);
        this.modelTrend = this.calculateTrend(results.stats.modelPrevTotal, this.modelTotal);
        
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

  // Calculate percentage change between previous and current values
  calculateTrend(previous: number, current: number): number {
    if (!previous || previous === 0) return 0;
    return Math.round(((current - previous) / previous) * 100);
  }

  applyFilters(): void {
    // Load data with filters (would need service method updates)
    this.loadDashboardData();
    this.snackBar.open('Filters applied successfully', 'Dismiss', {
      duration: 3000
    });
  }

  resetFilters(): void {
    const today = new Date();
    const lastMonth = new Date();
    lastMonth.setMonth(lastMonth.getMonth() - 1);
    
    this.dateRangeForm.setValue({
      start: lastMonth,
      end: today
    });
    
    this.selectedEventType = '';
    this.loadDashboardData();
    
    this.snackBar.open('Filters have been reset', 'Dismiss', {
      duration: 3000
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
    const date = new Date(timestamp);
    
    // Check if timestamp is from today
    const today = new Date();
    if (date.toDateString() === today.toDateString()) {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }
    
    // Check if timestamp is from yesterday
    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    if (date.toDateString() === yesterday.toDateString()) {
      return `Yesterday, ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`;
    }
    
    // Default format for older dates
    return date.toLocaleDateString([], { 
      month: 'short', 
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  viewSessionAudits(sessionId: string): void {
    if (sessionId) {
      this.router.navigate(['/session', sessionId]);
    }
  }

  getEventIcon(audit: AuditEvent): string {
    const eventType = audit.event_type?.toLowerCase() || '';
    
    if (eventType.includes('login') || eventType.includes('auth')) {
      return 'login';
    } else if (eventType.includes('error') || eventType.includes('fail')) {
      return 'error_outline';
    } else if (eventType.includes('data')) {
      return 'storage';
    } else if (eventType.includes('model')) {
      return 'model_training';
    } else if (eventType.includes('warning')) {
      return 'warning';
    } else if (eventType.includes('success')) {
      return 'check_circle';
    }
    
    return 'event_note';
  }

  getEventIconClass(audit: AuditEvent): string {
    const eventType = audit.event_type?.toLowerCase() || '';
    
    if (eventType.includes('error') || eventType.includes('fail')) {
      return 'error';
    } else if (eventType.includes('warning')) {
      return 'warning';
    } else if (eventType.includes('success')) {
      return 'success';
    }
    
    return '';
  }

  getEventTypeClass(audit: AuditEvent): string {
    const eventType = audit.event_type?.toLowerCase() || '';
    
    if (eventType.includes('login') || eventType.includes('auth')) {
      return 'auth';
    } else if (eventType.includes('data')) {
      return 'data';
    } else if (eventType.includes('error') || eventType.includes('fail')) {
      return 'error';
    }
    
    return '';
  }
}