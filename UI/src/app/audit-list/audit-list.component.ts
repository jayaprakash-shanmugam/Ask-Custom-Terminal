import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { MatPaginator } from '@angular/material/paginator';
import { MatSort } from '@angular/material/sort';
import { MatTableDataSource } from '@angular/material/table';
import { FormBuilder, FormGroup } from '@angular/forms';
import { MatSnackBar } from '@angular/material/snack-bar';
import { AuditEvent } from '../models/audit-event';
import { AuditService } from '../services/audit.service';

@Component({
  selector: 'app-audit-list',
  templateUrl: './audit-list.component.html',
  styleUrls: ['./audit-list.component.css']
})
export class AuditListComponent implements OnInit {
  @ViewChild(MatPaginator) paginator!: MatPaginator;
  @ViewChild(MatSort) sort!: MatSort;

  auditType: 'client' | 'model' = 'client';
  displayedColumns: string[] = ['event_type', 'message', 'timestamp', 'actions'];
  dataSource = new MatTableDataSource<AuditEvent>([]);
  totalItems = 0;
  currentPage = 0;
  pageSize = 10;
  pageSizeOptions = [5, 10, 25, 50];
  loading = true;
  filterForm: FormGroup;
  error: string | null = null;

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private auditService: AuditService,
    private fb: FormBuilder,
    private snackBar: MatSnackBar
  ) {
    this.filterForm = this.fb.group({
      eventType: [''],
      clientId: [''],
      sessionId: [''],
      startDate: [''],
      endDate: ['']
    });
  }

  ngOnInit(): void {
    this.route.data.subscribe(data => {
      this.auditType = data['type'] || 'client';
      this.loadAudits();
    });
  }

  loadAudits(): void {
    this.loading = true;
    this.error = null;
    
    const filters = this.filterForm.value;
    
    // Remove empty filters
    Object.keys(filters).forEach(key => {
      if (filters[key] === '' || filters[key] === null) {
        delete filters[key];
      }
    });
    
    // Convert dates to ISO strings if present
    if (filters.startDate) {
      filters.startDate = new Date(filters.startDate).toISOString();
    }
    if (filters.endDate) {
      filters.endDate = new Date(filters.endDate).toISOString();
    }

    const fetchAudits = this.auditType === 'client' ? 
      this.auditService.getClientAudits(this.currentPage + 1, this.pageSize, filters) :
      this.auditService.getModelAudits(this.currentPage + 1, this.pageSize, filters);

    fetchAudits.subscribe({
      next: (response) => {
        this.dataSource.data = response.data;
        this.totalItems = response.total;
        this.loading = false;
      },
      error: (err) => {
        this.error = `Failed to load ${this.auditType} audits. Please try again.`;
        this.loading = false;
        console.error(`Error loading ${this.auditType} audits:`, err);
      }
    });
  }

  onPageChange(event: any): void {
    this.currentPage = event.pageIndex;
    this.pageSize = event.pageSize;
    this.loadAudits();
  }

  applyFilter(): void {
    this.currentPage = 0;
    if (this.paginator) {
      this.paginator.firstPage();
    }
    this.loadAudits();
  }

  resetFilter(): void {
    this.filterForm.reset();
    this.currentPage = 0;
    if (this.paginator) {
      this.paginator.firstPage();
    }
    this.loadAudits();
  }

  viewDetails(audit: AuditEvent): void {
    this.router.navigate([`/${this.auditType}/${audit._id}`]);
  }
  viewSession(sessionId: string, event: Event): void {
    event.stopPropagation();
    if (sessionId) {
      this.router.navigate(['/session', sessionId]);
    }
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
}