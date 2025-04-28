import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { MatSnackBar } from '@angular/material/snack-bar';
import { AuditEvent } from '../models/audit-event';
import { AuditService } from '../services/audit.service';

@Component({
  selector: 'app-session-view',
  templateUrl: './session-view.component.html',
  styleUrls: ['./session-view.component.css']
})
export class SessionViewComponent implements OnInit {
  sessionId: string = '';
  events: AuditEvent[] = [];
  loading = true;
  error: string | null = null;
  displayedColumns: string[] = ['type', 'event_type', 'message', 'timestamp', 'actions'];

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private auditService: AuditService,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.route.params.subscribe(params => {
      this.sessionId = params['sessionId'];
      this.loadSessionEvents();
    });
  }

  loadSessionEvents(): void {
    this.loading = true;
    this.error = null;

    this.auditService.getSessionAudits(this.sessionId).subscribe({
      next: (response) => {
        this.events = response.data.sort((a, b) => 
          new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
        );
        this.loading = false;
      },
      error: (err) => {
        this.error = 'Failed to load session events. Please try again.';
        this.loading = false;
        console.error('Error loading session events:', err);
      }
    });
  }

  getEventSource(event: AuditEvent): string {
    // Determine if the event is from client or model collection
    // This might need adjustment based on how your data is structured
    return event.metadata && event.metadata['source'] === 'model' ? 'Model' : 'Client';
  }

  viewDetails(event: AuditEvent): void {
    const type = this.getEventSource(event).toLowerCase();
    this.router.navigate([`/${type}/${event._id}`]);
  }

  formatTimestamp(timestamp: Date): string {
    return new Date(timestamp).toLocaleString();
  }

  copyToClipboard(text: string, event: Event): void {
    event.stopPropagation();
    navigator.clipboard.writeText(text).then(() => {
      this.snackBar.open('Copied to clipboard', 'Dismiss', {
        duration: 2000,
      });
    });
  }

  goBack(): void {
    this.router.navigate(['/dashboard']);
  }
}