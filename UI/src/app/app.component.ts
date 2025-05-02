import { Component, ViewChild, OnInit, HostBinding, AfterViewInit } from '@angular/core';
import { MatSidenav } from '@angular/material/sidenav';
import { BreakpointObserver, Breakpoints } from '@angular/cdk/layout';
import { Observable } from 'rxjs';
import { map, shareReplay } from 'rxjs/operators';
import { Router, NavigationEnd, ActivatedRoute } from '@angular/router';
import { filter } from 'rxjs/operators';
import { Title } from '@angular/platform-browser';
import { trigger, transition, style, animate } from '@angular/animations';

interface BreadcrumbItem {
  label: string;
  url: string;
}

interface Notification {
  id: string;
  title: string;
  message: string;
  time: string;
  type: 'info' | 'warning' | 'alert' | 'success';
  icon: string;
  read: boolean;
}

interface SystemAlert {
  id: string;
  message: string;
  type: 'info' | 'warning' | 'error' | 'success';
}

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css'],
  animations: [
    trigger('fadeAnimation', [
      transition('* => *', [
        style({ opacity: 0, transform: 'translateY(10px)' }),
        animate('300ms ease-out', style({ opacity: 1, transform: 'translateY(0)' }))
      ])
    ])
  ]
})
export class AppComponent implements OnInit, AfterViewInit {
  @ViewChild('drawer') drawer!: MatSidenav;
  @HostBinding('class.dark-theme') isDarkTheme = false;

  title = 'Audit Dashboard';
  notificationCount: number = 5;
  isMinimized = false;
  searchExpanded = false;
  breadcrumbs: BreadcrumbItem[] = [];
  systemAlert: SystemAlert | null = null;
  
  navGroupsExpanded = {
    clients: true,
    analytics: true
  };
  
  notifications: Notification[] = [
    {
      id: '1',
      title: 'New Audit Request',
      message: 'Client XYZ has submitted a new audit request.',
      time: '10 minutes ago',
      type: 'info',
      icon: 'notification_important',
      read: false
    },
    {
      id: '2',
      title: 'Audit Completed',
      message: 'The audit for ABC Corp. has been completed.',
      time: '1 hour ago',
      type: 'success',
      icon: 'check_circle',
      read: false
    },
    {
      id: '3',
      title: 'System Update',
      message: 'System maintenance scheduled for tonight at 2 AM.',
      time: '3 hours ago',
      type: 'warning',
      icon: 'update',
      read: false
    },
    {
      id: '4',
      title: 'Critical Alert',
      message: 'Security vulnerability detected in Model #A-7291.',
      time: 'Yesterday',
      type: 'alert',
      icon: 'error',
      read: false
    },
    {
      id: '5',
      title: 'Welcome to the New Portal',
      message: 'Check out the new features and improved performance.',
      time: '2 days ago',
      type: 'info',
      icon: 'info',
      read: true
    }
  ];

  isHandset$: Observable<boolean> = this.breakpointObserver.observe(Breakpoints.Handset)
    .pipe(
      map((result) => result.matches),
      shareReplay()
    );

  constructor(
    private breakpointObserver: BreakpointObserver,
    private router: Router,
    private activatedRoute: ActivatedRoute,
    private titleService: Title
  ) {}

  ngOnInit() {
    // Initialize theme - check localStorage or default to light theme
    this.loadThemePreference();
    
    // Initialize system alert (example)
    this.systemAlert = {
      id: 'system-1',
      message: 'System maintenance scheduled for April 30, 2025. The portal will be unavailable from 02:00 to 04:00 UTC.',
      type: 'warning'
    };
    
    // Subscribe to router events to update breadcrumbs and title
    this.router.events.pipe(
      filter(event => event instanceof NavigationEnd)
    ).subscribe(() => {
      this.updateBreadcrumbs();
      this.updateTitle();
    });
    
    // Calculate notification count from hardcoded data
    this.notificationCount = this.notifications.filter(n => !n.read).length;
  }
  
  ngAfterViewInit() {
    // Subscribe to breakpoint changes for responsive adjustments
    this.isHandset$.subscribe(isHandset => {
      if (isHandset) {
        this.isMinimized = false;
      }
    });
  }

  /**
   * Load theme preference from localStorage
   */
  private loadThemePreference() {
    const storedTheme = localStorage.getItem('darkTheme');
    if (storedTheme) {
      this.isDarkTheme = storedTheme === 'true';
      this.applyTheme();
    }
  }

  /**
   * Apply current theme to document body
   */
  private applyTheme() {
    if (this.isDarkTheme) {
      document.body.classList.add('dark-theme');
    } else {
      document.body.classList.remove('dark-theme');
    }
  }

  /**
   * Close sidenav when clicking a link on mobile devices
   */
  closeNavOnMobile() {
    this.isHandset$.subscribe(isHandset => {
      if (isHandset) {
        this.drawer.close();
      }
    });
  }
  
  /**
   * Toggle the minimized state of the sidebar
   */
  toggleMinimized() {
    this.isMinimized = !this.isMinimized;
  }
  
  /**
   * Toggle navigation group expansion
   */
  toggleNavGroup(group: string) {
    this.navGroupsExpanded[group as keyof typeof this.navGroupsExpanded] = 
      !this.navGroupsExpanded[group as keyof typeof this.navGroupsExpanded];
  }
  
  /**
   * Toggle search input visibility
   */
  toggleSearch() {
    this.searchExpanded = !this.searchExpanded;
  }
  
  /**
   * Toggle between light and dark themes
   */
  toggleTheme() {
    this.isDarkTheme = !this.isDarkTheme;
    this.saveThemePreference();
    this.applyTheme();
  }
  
  /**
   * Save theme preference to localStorage
   */
  private saveThemePreference() {
    localStorage.setItem('darkTheme', this.isDarkTheme.toString());
  }

  /**
   * Mark all notifications as read
   */
  markAllAsRead() {
    // Hardcoded implementation
    this.notifications.forEach(notification => {
      notification.read = true;
    });
    this.notificationCount = 0;
  }
  
  /**
   * Mark a single notification as read
   */
  markAsRead(id: string) {
    const notification = this.notifications.find(n => n.id === id);
    if (notification && !notification.read) {
      notification.read = true;
      this.notificationCount -= 1;
    }
  }
  
  /**
   * Dismiss system alert
   */
  dismissAlert() {
    this.systemAlert = null;
  }
  
  /**
   * Update breadcrumbs based on current route
   */
  private updateBreadcrumbs() {
    let route = this.activatedRoute.root;
    this.breadcrumbs = [];
    
    // Build breadcrumbs array from route data
    this.buildBreadcrumbs(route);
  }
  
  /**
   * Recursively build breadcrumbs from route
   */
  private buildBreadcrumbs(route: ActivatedRoute, url: string = '', breadcrumbs: BreadcrumbItem[] = []): void {
    const children: ActivatedRoute[] = route.children;
    
    if (children.length === 0) {
      this.breadcrumbs = breadcrumbs;
      return;
    }
    
    for (const child of children) {
      const routeURL: string = child.snapshot.url.map(segment => segment.path).join('/');
      
      if (routeURL !== '') {
        url += `/${routeURL}`;
      }
      
      const label = child.snapshot.data['breadcrumb'];
      
      if (label) {
        breadcrumbs.push({ label, url });
      }
      
      this.buildBreadcrumbs(child, url, breadcrumbs);
    }
  }
  
  /**
   * Update page title based on activated route data
   */
  private updateTitle() {
    let route = this.activatedRoute;
    while (route.firstChild) {
      route = route.firstChild;
    }
    
    const routeTitle = route.snapshot.data['title'];
    if (routeTitle) {
      this.title = routeTitle;
      this.titleService.setTitle(`${routeTitle} - Audit Portal`);
    } else {
      this.title = 'Audit Dashboard';
      this.titleService.setTitle('Audit Portal');
    }
  }
  
  /**
   * Handle user logout
   */
  logout() {
    // Implement logout logic
    console.log('Logging out...');
    // Navigate to login page
    this.router.navigate(['/login']);
  }
}