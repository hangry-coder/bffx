import React, { useState, useMemo, useEffect, useCallback } from 'react';
import { ApiDocsPanel } from './ApiDocsPanel';
import { BffxDocsPanel } from './BffxDocsPanel';
import { AdminNav, type AdminPageId } from './AdminNav';
import { AnalyticsWidgetPlaceholder } from './components/AnalyticsWidgetPlaceholder';
import { FormField } from './components/FormField';
import { InfoCell } from './components/InfoCell';
import { KillSwitchControl } from './components/KillSwitchControl';
import { Modal } from './components/Modal';
import { PageHeader } from './components/PageHeader';
import { RelationPicker } from './components/RelationPicker';
import { RelationLink } from './components/RelationLink';
import { AssociationPanel } from './components/AssociationPanel';
import { useAdminIdleLogout } from './lib/adminIdleSession';
import { SidebarItem } from './components/SidebarItem';
import { StatCard } from './components/StatCard';
import {
  LayoutDashboard,
  Users,
  Activity,
  Bell,
  Settings,
  LogOut,
  Search,
  HelpCircle,
  Moon,
  Sun,
  Component as ComponentIcon,
  Code2,
  Radio,
  Download, 
  Monitor, 
  ShieldAlert,
  ChevronRight,
  MoreVertical,
  UserPlus,
  AlertTriangle,
  CheckCircle2,
  CloudDownload,
  Server,
  Flag,
  ShieldCheck,
  Layers,
  Plus,
  Filter,
  Edit2,
  Trash2,
  Save,
  X,
  Play,
  Square,
  Database,
  Lock,
  Cpu,
  RefreshCw,
  MapPin,
  Phone,
  Calendar,
  CreditCard,
  User as UserIcon,
  Laptop,
  Key,
  Smartphone,
  History,
  Zap,
  BarChart2,
  BookOpen,
  ChevronLeft,
  ChevronUp,
  ChevronDown,
  Eye
} from 'lucide-react';
import { 
  AreaChart, 
  Area, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  ResponsiveContainer,
  BarChart,
  Bar,
  Cell,
  PieChart,
  Pie,
  LineChart,
  Line,
  Legend
} from 'recharts';
import { motion, AnimatePresence } from 'motion/react';
import { cn } from './lib/utils';
import { humanizeResourceName } from './lib/labels';
import { type AnalyticsProviderInfo, type AnalyticsWidget } from './lib/dashboardAnalytics';
import { adminFetch, clearAuthIfUnauthorized } from './lib/adminSession';
import { menuContainsPage } from './lib/adminNavMenu';

// --- Types ---

type Page = AdminPageId;

type FeatureSource = {
  name: string;
  kind: 'builtin' | 'action' | 'resource' | 'unknown';
  action?: string;
  resource?: string;
  route_path?: string;
  route_method?: string;
  touches_resources?: string[];
};

type FeatureSection = {
  key: string;
  default_visible: boolean;
  ui_type?: string;
  layout?: string;
};

type KillSwitchState = {
  screen: string;
  section?: string;
  enabled: boolean;
  reason?: string;
  expires_at?: string;
  updated_by?: string;
  updated_at?: string;
};

type FeatureScreen = {
  name: string;
  nav_type?: string;
  icon?: string;
  order: number;
  requires_auth?: string;
  route_method?: string;
  route_path?: string;
  proto_service?: string;
  stream: boolean;
  sources: FeatureSource[];
  sections: FeatureSection[];
  touches_resources: string[];
  kill_switch?: KillSwitchState;
  section_kill_switches?: Record<string, KillSwitchState>;
};

type FeatureGroup = {
  name: string;
  display_name: string;
  screens: FeatureScreen[];
};

type FeatureOrphanAction = {
  name: string;
  group?: string;
  route_path?: string;
  route_method?: string;
  file?: string;
};

type FeatureActionRef = {
  name: string;
  route_method?: string;
  route_path?: string;
  file?: string;
  is_orphan: boolean;
};

type FeatureCluster = {
  name: string;
  display_name: string;
  group?: string;
  actions: FeatureActionRef[];
  touches_resources?: string[];
  screens_using?: string[];
};

type FeatureTree = {
  groups: FeatureGroup[];
  orphan_actions: FeatureOrphanAction[];
  feature_clusters: FeatureCluster[];
};

interface User {
  id: string;
  name: string;
  email: string;
  role: 'User' | 'Admin' | 'Moderator';
  status: 'Active' | 'Inactive' | 'Pending';
  lastActive: string;
  phone?: string;
  joinDate?: string;
  location?: string;
  subscription?: string;
  bio?: string;
  preferences?: {
    notifications: boolean;
    marketing: boolean;
    theme: 'Dark' | 'Light' | 'System';
  };
  devices?: { type: 'iOS' | 'Android' | 'Web', name: string, lastUsed: string }[];
}

interface Order {
  id: string;
  userId: string;
  amount: number;
  date: string;
  status: 'Completed' | 'Processing' | 'Refunded';
}

interface FeatureFlag {
  id: string;
  key: string;
  description: string;
  enabled: boolean;
  environment: 'Production' | 'Staging' | 'Development';
  targeting?: {
    users: string[];
    segments: string[];
    rollout: number;
  };
}

interface Resource {
  id: string;
  [key: string]: any;
}

interface Collection {
  id: string;
  name: string;
  displayName?: string;
  fields: { name: string; type: string }[];
  data: Resource[];
  pinnedToMenu?: boolean;
  spec?: any;
}

interface AdminUser {
  id: string;
  name: string;
  email: string;
  tier: string;
  time: string;
}

// --- Mock Data ---

const INITIAL_USERS: User[] = [
  { 
    id: '1', 
    name: 'Jane Cooper', 
    email: 'jane.cooper@example.com', 
    role: 'Admin', 
    status: 'Active', 
    lastActive: '2 mins ago',
    phone: '+1 (555) 123-4567',
    joinDate: 'Jan 12, 2023',
    location: 'San Francisco, CA',
    subscription: 'Pro Plan',
    bio: 'Senior Infrastructure Engineer focusing on spatial computing and neural networks.',
    preferences: { notifications: true, marketing: false, theme: 'Dark' },
    devices: [
      { type: 'iOS', name: 'iPhone 15 Pro', lastUsed: 'Just now' },
      { type: 'Web', name: 'MacBook Pro 16"', lastUsed: '5 mins ago' }
    ]
  },
  { 
    id: '2', 
    name: 'Cody Fisher', 
    email: 'cody.fisher@example.com', 
    role: 'User', 
    status: 'Active', 
    lastActive: '5 hours ago',
    phone: '+44 7712 345678',
    joinDate: 'Mar 05, 2024',
    location: 'London, UK',
    subscription: 'Free Tier',
    devices: [{ type: 'Android', name: 'Pixel 8', lastUsed: '4 hours ago' }]
  },
  { id: '3', name: 'Esther Howard', email: 'esther.howard@example.com', role: 'Moderator', status: 'Inactive', lastActive: '2 days ago' },
  { id: '4', name: 'Jenny Wilson', email: 'jenny.wilson@example.com', role: 'User', status: 'Pending', lastActive: 'Never' },
  { id: '5', name: 'Robert Fox', email: 'robert.fox@example.com', role: 'User', status: 'Active', lastActive: '12 mins ago' },
  { id: '6', name: 'Kristin Watson', email: 'kristin.watson@example.com', role: 'Admin', status: 'Active', lastActive: 'Just now' },
];

const MOCK_ORDERS: Order[] = [
  { id: 'ord_1', userId: '1', amount: 199.99, date: '2024-03-15', status: 'Completed' },
  { id: 'ord_2', userId: '1', amount: 49.00, date: '2024-03-22', status: 'Processing' },
  { id: 'ord_3', userId: '2', amount: 29.99, date: '2024-02-10', status: 'Completed' },
];

const INITIAL_ADMINS: AdminUser[] = [
  { id: 'a1', name: 'Root System', email: 'root@internal.obsidian.io', tier: 'Superuser', time: 'Active Now' },
  { id: 'a2', name: 'Sec-Ops Manager', email: 'sec@internal.obsidian.io', tier: 'Security', time: '2 hours ago' },
  { id: 'a3', name: 'Dev-Ops Lead', email: 'ops@internal.obsidian.io', tier: 'Full Control', time: '8 mins ago' },
];

const MOCK_COLLECTIONS: Collection[] = [
  {
    id: 'product_catalog',
    name: 'Product_Catalog',
    pinnedToMenu: true,
    fields: [
      { name: 'name', type: 'text' },
      { name: 'sku', type: 'text' },
      { name: 'price', type: 'number' },
      { name: 'stock', type: 'number' },
    ],
    data: [
      { id: 'p1', name: 'Neural Processor X1', sku: 'NP-X1-001', price: 299.99, stock: 45 },
      { id: 'p2', name: 'Quantum Core', sku: 'QC-009', price: 899.00, stock: 12 },
    ]
  },
  {
    id: 'api_keys',
    name: 'Internal_Audit',
    pinnedToMenu: false,
    fields: [
      { name: 'action', type: 'text' },
      { name: 'performed_by', type: 'text' },
      { name: 'timestamp', type: 'text' },
    ],
    data: [
      { id: 'a1', action: 'Login Success', performed_by: 'jane.cooper@example.com', timestamp: '2024-03-20 10:05:32' },
      { id: 'a2', action: 'Flag Updated', performed_by: 'root@internal.io', timestamp: '2024-03-20 11:42:10' },
    ]
  }
];

const INITIAL_FLAGS: FeatureFlag[] = [
  { 
    id: '1', 
    key: 'dark_mode_v2', 
    description: 'Enable the new dark mode engine', 
    enabled: true, 
    environment: 'Production',
    targeting: { users: ['user_123', 'user_456'], segments: ['Beta'], rollout: 100 }
  },
  { id: '2', key: 'api_v3_beta', description: 'Enable beta testing for API v3', enabled: false, environment: 'Staging' },
  { id: '3', key: 'user_profile_redesign', description: 'Show redesigned profile pages', enabled: true, environment: 'Development' },
  { id: '4', key: 'smart_search_ai', description: 'Rollout Gemini-powered search', enabled: false, environment: 'Production' },
];

const PERFORMANCE_DATA = [
  { time: '00:00', traffic: 400, load: 24, latency: 120, errors: 2, users: 1500, cpu: 12, mem: 45 },
  { time: '04:00', traffic: 300, load: 19, latency: 140, errors: 5, users: 1200, cpu: 8, mem: 42 },
  { time: '08:00', traffic: 900, load: 45, latency: 180, errors: 12, users: 4500, cpu: 34, mem: 58 },
  { time: '12:00', traffic: 1200, load: 39, latency: 150, errors: 8, users: 5120, cpu: 48, mem: 62 },
  { time: '16:00', traffic: 800, load: 48, latency: 165, errors: 15, users: 3800, cpu: 28, mem: 55 },
  { time: '20:00', traffic: 1100, load: 30, latency: 130, errors: 4, users: 4800, cpu: 38, mem: 59 },
  { time: '23:59', traffic: 700, load: 43, latency: 145, errors: 3, users: 3200, cpu: 22, mem: 51 },
];

const ALERTS_DATA = [
  { id: 1, type: 'critical', msg: 'Database connection failed', source: 'SQL-PROD-01', time: '5m' },
  { id: 2, type: 'warning', msg: 'High CPU usage detected', source: 'Web-Node-12', time: '12m' },
  { id: 3, type: 'info', msg: 'Weekly backup started', source: 'System', time: '1h' },
  { id: 4, type: 'critical', msg: 'API Rate limit exceeded for User: ID_992', source: 'Gateway', time: '2h' },
];

// --- Main App Component ---

export default function App() {
  const API_BASE = (() => {
    const path = window.location.pathname;
    const m = path.match(/^(.*\/admin)(\/.*)?$/);
    return m ? m[1] : '';
  })();
  const ADMIN_API = `${API_BASE}/api/admin`;

  const [theme, setTheme] = useState<'light' | 'dark'>('dark');
  const [activePage, setActivePage] = useState<Page>('dashboard');

  useEffect(() => {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }, [theme]);
  const [systemSubTab, setSystemSubTab] = useState<'keys' | 'admins' | 'workers' | 'jobs' | 'security'>('keys');
  const [jobs, setJobs] = useState<any[]>([]);
  const [isLoadingJobs, setIsLoadingJobs] = useState<boolean>(false);
  const [appSubTab, setAppSubTab] = useState<'general' | 'onboarding' | 'auth'>('general');
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(false);
  const [authChecked, setAuthChecked] = useState<boolean>(false);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [users, setUsers] = useState<User[]>([]);
  const [adminUsers, setAdminUsers] = useState<AdminUser[]>([]);
  const [metricsSummary, setMetricsSummary] = useState<any>({
    total_requests: 0,
    error_rate: 0,
    avg_latency_ms: 0,
    active_sessions: 0,
    uptime: '0s'
  });
  const [dashboardWidgets, setDashboardWidgets] = useState<any[]>([]);
  const [auditLogs, setAuditLogs] = useState<any[]>([]);
  const [flags, setFlags] = useState<FeatureFlag[]>([]);
  const [collections, setCollections] = useState<Collection[]>([]);
  const [adminConfig, setAdminConfig] = useState<any>(null);
  const [search, setSearch] = useState('');
  
  // Flag View State
  const [selectedFlagId, setSelectedFlagId] = useState<string | null>(null);
  const [targetingEdit, setTargetingEdit] = useState<{
    users: string[];
    segments: string[];
    rollout: number;
  } | null>(null);
  const [newUserKey, setNewUserKey] = useState('');

  // Resource View State
  const [selectedCollectionId, setSelectedCollectionId] = useState<string>('');

  // Resource pagination & query states
  const [resourceQueryParams, setResourceQueryParams] = useState<{
    page: number;
    perPage: number;
    sort: string;
    scope: string;
    filters: Record<string, string>;
  }>({
    page: 1,
    perPage: 25,
    sort: '',
    scope: 'all',
    filters: {}
  });
  const [resourceTotalCount, setResourceTotalCount] = useState<number>(0);
  const [isFilterSidebarOpen, setIsFilterSidebarOpen] = useState<boolean>(false);
  const [localFilters, setLocalFilters] = useState<Record<string, string>>({});

  // Reset resource parameters when selectedCollectionId changes
  useEffect(() => {
    if (!selectedCollectionId) return;
    const coll = collections.find(c => c.id === selectedCollectionId);
    let defaultScope = 'all';
    if (coll && coll.spec && coll.spec.index && Array.isArray(coll.spec.index.scopes)) {
      const defaultS = coll.spec.index.scopes.find((s: any) => s.default);
      if (defaultS) {
        defaultScope = defaultS.name;
      }
    }
    let defaultSort = '';
    if (coll && coll.spec && coll.spec.index && coll.spec.index.default_sort) {
      defaultSort = coll.spec.index.default_sort;
    }
    let defaultPerPage = 25;
    if (coll && coll.spec && coll.spec.index && coll.spec.index.per_page) {
      defaultPerPage = coll.spec.index.per_page;
    } else if (adminConfig && adminConfig.site && adminConfig.site.default_per_page) {
      defaultPerPage = adminConfig.site.default_per_page;
    }

    setResourceQueryParams({
      page: 1,
      perPage: defaultPerPage,
      sort: defaultSort,
      scope: defaultScope,
      filters: {}
    });
    setResourceTotalCount(0);
    setLocalFilters({});
    setSelectedRowIds([]);
  }, [selectedCollectionId]);

  // Modal States
  const [isUserModalOpen, setIsUserModalOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [isAdminModalOpen, setIsAdminModalOpen] = useState(false);
  const [editingAdmin, setEditingAdmin] = useState<AdminUser | null>(null);
  const [isFlagModalOpen, setIsFlagModalOpen] = useState(false);
  const [editingFlag, setEditingFlag] = useState<FeatureFlag | null>(null);
  const [isResourceModalOpen, setIsResourceModalOpen] = useState(false);
  const [editingResource, setEditingResource] = useState<Resource | null>(null);
  const [showingResource, setShowingResource] = useState<any>(null);
  const [isShowModalOpen, setIsShowModalOpen] = useState(false);
  const [selectedRowIds, setSelectedRowIds] = useState<string[]>([]);
  const [incidents, setIncidents] = useState<any[]>([]);
  const [perfData, setPerfData] = useState<any[]>(PERFORMANCE_DATA);
  const [providers, setProviders] = useState<any>({
    telemetry: { name: 'battery', capabilities: ['write', 'read.summary', 'read.stream'] },
    incident: { name: 'battery', capabilities: ['write', 'read.list', 'read.summary', 'resolve'] },
    analytics: { name: 'battery', capabilities: ['write', 'read.list', 'read.summary'] },
    audit: { name: 'battery', capabilities: ['write', 'read.list', 'read.summary'] }
  });

  // Form States (simplification)
  const [formData, setFormData] = useState<any>({});

  const [userDetail, setUserDetail] = useState<any>(null);

  // App Features State
  const [featureTree, setFeatureTree] = useState<FeatureTree>({ groups: [], orphan_actions: [], feature_clusters: [] });
  const [featureTreeError, setFeatureTreeError] = useState<string | null>(null);
  const [featureTreeLoading, setFeatureTreeLoading] = useState<boolean>(false);
  const [selectedScreenName, setSelectedScreenName] = useState<string | null>(null);
  const [selectedClusterName, setSelectedClusterName] = useState<string | null>(null);
  const [dismissedOrphanBanner, setDismissedOrphanBanner] = useState<boolean>(false);
  const [runAsUserId, setRunAsUserId] = useState<string>('');
  const [runAsLocale, setRunAsLocale] = useState<string>('');
  const [runAsResult, setRunAsResult] = useState<any | null>(null);
  const [runAsError, setRunAsError] = useState<string | null>(null);
  const [runAsLoading, setRunAsLoading] = useState<boolean>(false);

  useEffect(() => {
    setRunAsResult(null);
    setRunAsError(null);
  }, [selectedScreenName]);

  const refreshFeatureTree = () => {
    setFeatureTreeLoading(true);
    setFeatureTreeError(null);
    fetch(`${ADMIN_API}/features`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        if (!res.ok) throw new Error(`features ${res.status}`);
        return res.json();
      })
      .then((data: FeatureTree) => {
        const groups = Array.isArray(data?.groups) ? data.groups : [];
        const orphans = Array.isArray(data?.orphan_actions) ? data.orphan_actions : [];
        const clusters = Array.isArray(data?.feature_clusters) ? data.feature_clusters : [];
        setFeatureTree({ groups, orphan_actions: orphans, feature_clusters: clusters });
      })
      .catch(err => {
        setFeatureTreeError(err?.message || 'failed to fetch features');
      })
      .finally(() => setFeatureTreeLoading(false));
  };

  const fetchAdminUsers = () => {
    fetch(`${ADMIN_API}/resources/AdminUser`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        return res.json();
      })
      .then(data => {
        if (Array.isArray(data)) {
          setAdminUsers(data.map((u: any) => ({
            id: u.id,
            name: u.email ? u.email.split('@')[0] : 'Admin',
            email: u.email || '',
            tier: u.role || 'Superuser',
            time: u.updated_at ? new Date(u.updated_at).toLocaleDateString() : 'Active Now'
          })));
        }
      })
      .catch(err => console.error("failed to fetch admin users", err));
  };

  const fetchMetricsSummary = () => {
    fetch(`${ADMIN_API}/metrics/summary`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        return res.json();
      })
      .then(data => {
        if (data && typeof data === 'object') {
          setMetricsSummary(data);
        }
      })
      .catch(err => console.error("failed to fetch metrics summary", err));
  };

  const fetchDashboardData = () => {
    fetch(`${ADMIN_API}/dashboard/data`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        if (!res.ok) throw new Error("Failed to fetch dashboard data");
        return res.json();
      })
      .then(data => {
        if (data && Array.isArray(data.widgets)) {
          setDashboardWidgets(data.widgets);
        }
      })
      .catch(err => console.error("failed to fetch dashboard data", err));
  };

  const handleDeleteAdmin = (admin: AdminUser) => {
    if (window.confirm(`Are you sure you want to revoke administrative access for ${admin.email}?`)) {
      fetch(`${ADMIN_API}/resources/AdminUser/${admin.id}`, { method: 'DELETE' })
      .then(async res => {
        if (res.status === 400) {
          const errData = await res.json();
          alert(errData.error || "Cannot delete admin user");
          return;
        }
        if (!res.ok) throw new Error("Failed to delete admin user");
        fetchAdminUsers();
      })
      .catch(err => console.error("failed to delete admin", err));
    }
  };

  const [cacheBusy, setCacheBusy] = useState<string | null>(null);
  const [cacheMsg, setCacheMsg] = useState<string | null>(null);

  type ScreenMetricsResp = {
    screen: string;
    provider?: { display_name?: string; name?: string; vendor_url?: string; capabilities?: string[] };
    samples?: Array<{ time: string; traffic?: number; latency?: number; errors?: number; users?: number }>;
  };
  const [observeMetrics, setObserveMetrics] = useState<ScreenMetricsResp | null>(null);
  const [observeLoading, setObserveLoading] = useState<boolean>(false);

  useEffect(() => {
    if (!isAuthenticated || !selectedScreenName) {
      setObserveMetrics(null);
      return;
    }
    setObserveLoading(true);
    fetch(`${ADMIN_API}/features/screens/${encodeURIComponent(selectedScreenName)}/metrics`)
      .then(async res => {
        if (!res.ok) throw new Error(await res.text() || `HTTP ${res.status}`);
        return res.json();
      })
      .then((data: ScreenMetricsResp) => setObserveMetrics(data))
      .catch(() => setObserveMetrics(null))
      .finally(() => setObserveLoading(false));
  }, [isAuthenticated, selectedScreenName, ADMIN_API]);

  const flushScreenCache = (screenName: string, sectionKey?: string) => {
    const id = sectionKey ? `${screenName}/${sectionKey}` : screenName;
    setCacheBusy(id);
    setCacheMsg(null);
    const url = sectionKey
      ? `${ADMIN_API}/features/screens/${encodeURIComponent(screenName)}/sections/${encodeURIComponent(sectionKey)}/cache/invalidate`
      : `${ADMIN_API}/features/screens/${encodeURIComponent(screenName)}/cache/invalidate`;
    fetch(url, { method: 'POST' })
      .then(async res => {
        if (!res.ok) throw new Error(await res.text() || `HTTP ${res.status}`);
        return res.json();
      })
      .then(data => setCacheMsg(`Flushed ${data?.flushed ?? 0} cache entries for ${id}`))
      .catch(err => setCacheMsg(`Failed: ${err?.message || String(err)}`))
      .finally(() => setCacheBusy(null));
  };

  const toggleKillSwitch = (
    screenName: string,
    sectionKey: string | undefined,
    enabled: boolean,
    reason: string,
    expiresIn: string,
  ) => {
    const url = sectionKey
      ? `${ADMIN_API}/features/screens/${encodeURIComponent(screenName)}/sections/${encodeURIComponent(sectionKey)}`
      : `${ADMIN_API}/features/screens/${encodeURIComponent(screenName)}`;
    const body: any = { enabled, reason };
    if (expiresIn) body.expires_in = expiresIn;
    return fetch(url, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
      .then(async res => {
        if (!res.ok) throw new Error(await res.text() || `HTTP ${res.status}`);
        return res.json();
      })
      .then(() => refreshFeatureTree());
  };

  const handleRunAs = (screenName: string) => {
    setRunAsLoading(true);
    setRunAsError(null);
    setRunAsResult(null);
    const body: any = {};
    if (runAsUserId.trim()) body.user_id = runAsUserId.trim();
    if (runAsLocale.trim()) body.locale = runAsLocale.trim();
    fetch(`${ADMIN_API}/features/screens/${encodeURIComponent(screenName)}/run-as`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
      .then(async res => {
        const text = await res.text();
        if (!res.ok) throw new Error(text || `HTTP ${res.status}`);
        try { return JSON.parse(text); } catch { return { raw: text }; }
      })
      .then(data => setRunAsResult(data))
      .catch(err => setRunAsError(err?.message || String(err)))
      .finally(() => setRunAsLoading(false));
  };

  const [settings, setSettings] = useState<Record<string, string>>({
    maintenance: 'false',
    force_update: 'true',
    push_notifs: 'true',
    analytics: 'true',
    min_ios_build: '14.0.1',
    min_android_sdk: '23.0',
    theme_override: 'Dark',
    welcome_copy: 'Experience the next generation of spatial computing with Obsidian Pro. Secure, fast, and distributed at the edge.',
    cta_button_text: 'Initialize Identity',
    accent_color: '#3b82f6',
    disable_guest_signup: 'false',
    biometric_enforcement: 'true',
  });

  const saveSettings = () => {
    fetch(`${ADMIN_API}/settings`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    })
      .then(res => {
        if (res.ok) {
          alert('Configuration deployed successfully!');
        } else {
          alert('Failed to deploy configuration.');
        }
      })
      .catch(err => {
        console.error(err);
        alert('Error deploying configuration.');
      });
  };

  // Bootstrap auth from session endpoint (HttpOnly cookie; do not use /metrics or 403 as logout).
  useEffect(() => {
    adminFetch(`${ADMIN_API}/session`)
      .then((res) => {
        setIsAuthenticated(res.ok);
      })
      .catch(() => {
        /* network blip — do not force login until we know the session is gone */
      })
      .finally(() => setAuthChecked(true));
  }, [ADMIN_API]);

  // Keepalive: only 401 clears auth (avoids false login screen on transient errors).
  useEffect(() => {
    if (!isAuthenticated) return;
    const interval = setInterval(() => {
      adminFetch(`${ADMIN_API}/session`).then((res) => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
      });
    }, 5 * 60 * 1000);
    return () => clearInterval(interval);
  }, [isAuthenticated, ADMIN_API]);

  // Auth & Login helper
  const handleLogin = (e: React.FormEvent) => {
    e.preventDefault();
    const email = formData.username || '';
    const password = formData.password || '';
    fetch(`${ADMIN_API}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password })
    })
    .then(async res => {
      if (res.ok) {
        setIsAuthenticated(true);
        setLoginError(null);
        return;
      }
      if (res.status === 403) {
        let msg = "This account cannot access the admin panel. Use admin@example.com (superadmin) from project bootstrap.";
        try {
          const body = await res.json();
          if (typeof body?.error === 'string') msg = body.error;
        } catch { /* ignore */ }
        setLoginError(msg);
        return;
      }
      setLoginError("Invalid admin credentials");
    })
    .catch(err => {
      console.error("Login request failed", err);
      setLoginError("Network connection error");
    });
  };

  const handleLogout = () => {
    fetch(`${ADMIN_API}/logout`, { method: 'POST', credentials: 'same-origin' })
      .catch(() => { /* even if it fails, force the UI back to login */ })
      .finally(() => {
        setIsAuthenticated(false);
        window.location.reload();
      });
  };

  const openRelatedResource = useCallback(
    (resource: string, id: string) => {
      setActivePage('resources');
      setSelectedCollectionId(resource);
      adminFetch(`${ADMIN_API}/resources/${encodeURIComponent(resource)}/${encodeURIComponent(id)}`)
        .then((res) => (res.ok ? res.json() : null))
        .then((row) => {
          if (row) {
            setShowingResource(row);
            setIsShowModalOpen(true);
          }
        })
        .catch((err) => console.error('failed to open related resource', err));
    },
    [ADMIN_API],
  );

  const idleTimeoutSec = adminConfig?.site?.session_resolved?.idle_timeout_seconds ?? 0;
  useAdminIdleLogout({
    enabled: isAuthenticated && idleTimeoutSec > 0,
    idleTimeoutSeconds: idleTimeoutSec,
    onIdle: handleLogout,
  });

  const loadJobs = () => {
    setIsLoadingJobs(true);
    fetch(`${ADMIN_API}/jobs`)
      .then(res => {
        if (!res.ok) throw new Error("HTTP error " + res.status);
        return res.json();
      })
      .then(data => {
        setJobs(Array.isArray(data) ? data : []);
      })
      .catch(err => {
        console.error("failed to fetch jobs", err);
      })
      .finally(() => {
        setIsLoadingJobs(false);
      });
  };

  const handleRetryJob = (id: string) => {
    fetch(`${ADMIN_API}/jobs/${encodeURIComponent(id)}/retry`, { method: 'POST' })
      .then(res => {
        if (res.ok) {
          loadJobs();
        } else {
          alert("Failed to retry job");
        }
      })
      .catch(err => console.error("retry job failed", err));
  };

  const handleCancelJob = (id: string) => {
    fetch(`${ADMIN_API}/jobs/${encodeURIComponent(id)}/cancel`, { method: 'POST' })
      .then(res => {
        if (res.ok) {
          loadJobs();
        } else {
          alert("Failed to cancel job");
        }
      })
      .catch(err => console.error("cancel job failed", err));
  };

  useEffect(() => {
    if (!isAuthenticated) return;
    if (activePage === 'system-settings' && systemSubTab === 'jobs') {
      loadJobs();
    }
  }, [isAuthenticated, activePage, systemSubTab]);

  // Load backend data dynamically
  useEffect(() => {
    if (!isAuthenticated) return;

    fetchAdminUsers();

    // 1. Load users
    fetch(`${ADMIN_API}/users`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        return res.json();
      })
      .then(data => {
        if (Array.isArray(data)) {
          setUsers(data.map((u: any) => ({
            id: u.id,
            name: u.name || u.email || 'Anonymous',
            email: u.email || '',
            role: (u.role === 'Admin' || u.role === 'admin' ? 'Admin' : 'User') as 'User' | 'Admin' | 'Moderator',
            status: (u.status === 'Inactive' || u.status === 'inactive' ? 'Inactive' : u.status === 'Pending' || u.status === 'pending' ? 'Pending' : 'Active') as 'Active' | 'Inactive' | 'Pending',
            lastActive: u.updated_at ? new Date(u.updated_at).toLocaleDateString() : 'Active'
          })));
        }
      })
      .catch(err => console.error("failed to fetch users", err));

    // 2. Load flags
    fetch(`${ADMIN_API}/flags`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        return res.json();
      })
      .then(data => {
        if (Array.isArray(data)) {
          setFlags(data.map((f: any) => ({
            id: f.key,
            key: f.key,
            description: f.description || '',
            enabled: f.enabled || false,
            environment: 'Production' as const,
            targeting: f.targeting || { users: [], segments: [], rollout: 0 }
          })));
        }
      })
      .catch(err => console.error("failed to fetch flags", err));

    // 3. Load Admin Config
    fetch(`${ADMIN_API}/config`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        if (!res.ok) throw new Error("Failed to fetch admin config");
        return res.json();
      })
      .then(config => {
        setAdminConfig(config);
        const allResources: any[] = [];
        if (config && Array.isArray(config.resources)) {
          config.resources.forEach((r: any) => {
            if (r.enabled === false) return;
            allResources.push({
              id: r.resource,
              name: r.resource,
              displayName: r.menu?.label || r.resource,
              fields: r.fields || [],
              data: [],
              pinnedToMenu: !!r.menu?.pin,
              spec: r
            });
          });
        }
        allResources.sort((a, b) => a.name.localeCompare(b.name));
        setCollections(allResources);
        if (allResources.length > 0) {
          setSelectedCollectionId(allResources[0].id);
        }
      })
      .catch(err => console.error("failed to fetch admin config", err));

    // 4. Load settings
    fetch(`${ADMIN_API}/settings`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        if (!res.ok) throw new Error("Failed to fetch settings");
        return res.json();
      })
      .then(data => {
        if (data && typeof data === 'object') {
          setSettings(prev => ({ ...prev, ...data }));
        }
      })
      .catch(err => console.error("failed to fetch settings", err));

    // 5. Load incidents
    if (providers.incident?.capabilities?.includes('read.list')) {
      fetch(`${ADMIN_API}/incidents`)
        .then(res => {
          clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
          if (!res.ok) throw new Error("Failed to fetch incidents");
          return res.json();
        })
        .then(data => {
          if (Array.isArray(data)) {
            setIncidents(data);
          }
        })
        .catch(err => console.error("failed to fetch incidents", err));
    }

    // 6. Load providers info
    fetch(`${ADMIN_API}/providers`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        if (!res.ok) throw new Error("Failed to fetch providers");
        return res.json();
      })
      .then(data => {
        if (data && typeof data === 'object') {
          setProviders(data);
        }
      })
      .catch(err => console.error("failed to fetch providers", err));

    // 7. Load App Features tree
    setFeatureTreeLoading(true);
    setFeatureTreeError(null);
    fetch(`${ADMIN_API}/features`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        if (!res.ok) throw new Error(`features ${res.status}`);
        return res.json();
      })
      .then((data: FeatureTree) => {
        const groups = Array.isArray(data?.groups) ? data.groups : [];
        const orphans = Array.isArray(data?.orphan_actions) ? data.orphan_actions : [];
        const clusters = Array.isArray(data?.feature_clusters) ? data.feature_clusters : [];
        setFeatureTree({ groups, orphan_actions: orphans, feature_clusters: clusters });
        if (!selectedScreenName && groups.length > 0 && groups[0].screens.length > 0) {
          setSelectedScreenName(groups[0].screens[0].name);
        }
      })
      .catch(err => {
        setFeatureTreeError(err?.message || 'failed to fetch features');
        console.error('failed to fetch features', err);
      })
      .finally(() => setFeatureTreeLoading(false));
  }, [isAuthenticated]);

  // Fetch records of selected resource
  useEffect(() => {
    if (!isAuthenticated || !selectedCollectionId) return;

    const params = new URLSearchParams();
    params.append('page', String(resourceQueryParams.page));
    params.append('per_page', String(resourceQueryParams.perPage));
    if (resourceQueryParams.sort) {
      params.append('sort', resourceQueryParams.sort);
    }
    if (resourceQueryParams.scope) {
      params.append('scope', resourceQueryParams.scope);
    }
    Object.entries(resourceQueryParams.filters).forEach(([k, v]) => {
      if (v) {
        params.append(k, v);
      }
    });

    fetch(`${ADMIN_API}/resources/${encodeURIComponent(selectedCollectionId)}?${params.toString()}`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        
        const total = res.headers.get('X-Total-Count');
        if (total) {
          setResourceTotalCount(parseInt(total, 10));
        } else {
          setResourceTotalCount(0);
        }

        return res.json();
      })
      .then(data => {
        if (Array.isArray(data)) {
          setResourceTotalCount(prev => prev || data.length);
          setCollections(prev => prev.map(c => {
            if (c.id === selectedCollectionId) {
              return { ...c, data: data };
            }
            return c;
          }));
        }
      })
      .catch(err => console.error("failed to fetch records for " + selectedCollectionId, err));
  }, [selectedCollectionId, isAuthenticated, resourceQueryParams]);

  // SSE metrics streaming connection
  useEffect(() => {
    if (!isAuthenticated) return;
    if (!providers.telemetry?.capabilities?.includes('read.stream')) return;

    const eventSource = new EventSource(`${ADMIN_API}/metrics/stream`);

    eventSource.onmessage = (event) => {
      try {
        const sample = JSON.parse(event.data);
        if (sample && sample.time) {
          setPerfData(prev => {
            const updated = [...prev, sample];
            if (updated.length > 7) {
              updated.shift();
            }
            return updated;
          });
        }
      } catch (err) {
        console.error("failed to parse SSE event data", err);
      }
    };

    eventSource.onerror = (err) => {
      console.warn("EventSource failed, closing connection:", err);
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  }, [isAuthenticated, ADMIN_API, providers]);

  // Poll metrics summary and dashboard data on a 30-second interval
  useEffect(() => {
    if (!isAuthenticated) return;
    fetchMetricsSummary();
    fetchDashboardData();
    const interval = setInterval(() => {
      fetchMetricsSummary();
      fetchDashboardData();
    }, 30000);
    return () => clearInterval(interval);
  }, [isAuthenticated, ADMIN_API]);

  // Fetch single user detail
  useEffect(() => {
    if (!isAuthenticated || !selectedUserId) {
      setUserDetail(null);
      return;
    }
    fetch(`${ADMIN_API}/users/${encodeURIComponent(selectedUserId)}`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        return res.json();
      })
      .then(data => {
        setUserDetail(data);
      })
      .catch(err => console.error("failed to fetch user detail", err));

    fetch(`${ADMIN_API}/resources/AuditLog`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        return res.json();
      })
      .then(data => {
        if (Array.isArray(data)) setAuditLogs(data);
      })
      .catch(err => console.error("failed to fetch audit logs", err));
  }, [selectedUserId, isAuthenticated]);

  const activeCollection = useMemo(() => collections.find(c => c.id === selectedCollectionId) || collections[0] || { id: '', name: '', fields: [], data: [], pinnedToMenu: false }, [collections, selectedCollectionId]);
  const visibleFields = useMemo(() => {
    if (!activeCollection) return [];
    const fields = activeCollection.fields || [];
    const columns = activeCollection.spec?.index?.columns;
    if (columns && columns.length > 0) {
      return fields.filter(f => columns.includes(f.name));
    }
    return fields.filter(f => f.name !== 'id');
  }, [activeCollection]);
  const activeFlag = useMemo(() => flags.find(f => f.id === selectedFlagId), [flags, selectedFlagId]);

  useEffect(() => {
    if (activeFlag) {
      setTargetingEdit({
        users: activeFlag.targeting?.users || [],
        segments: activeFlag.targeting?.segments || [],
        rollout: activeFlag.targeting?.rollout || 0
      });
    } else {
      setTargetingEdit(null);
    }
  }, [activeFlag]);

  const filteredUsers = useMemo(() => {
    return users.filter(u => 
      u.name.toLowerCase().includes(search.toLowerCase()) || 
      u.email.toLowerCase().includes(search.toLowerCase())
    );
  }, [users, search]);

  const handleEditUser = (user: User) => {
    setEditingUser(user);
    setFormData(user);
    setIsUserModalOpen(true);
  };

  const saveUser = () => {
    const payload = {
      ...formData,
      email: formData.email,
      name: formData.name,
      role: formData.role || 'User',
      status: formData.status || 'Active'
    };
    if (editingUser) {
      fetch(`${ADMIN_API}/resources/user/${encodeURIComponent(editingUser.id)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
      .then(res => res.json())
      .then(updated => {
        setUsers(users.map(u => u.id === editingUser.id ? { 
          ...u, 
          name: updated.name || updated.email || 'Anonymous',
          email: updated.email || '',
          role: (updated.role === 'Admin' || updated.role === 'admin' ? 'Admin' : 'User') as 'User' | 'Admin' | 'Moderator',
          status: (updated.status === 'Inactive' || updated.status === 'inactive' ? 'Inactive' : updated.status === 'Pending' || updated.status === 'pending' ? 'Pending' : 'Active') as 'Active' | 'Inactive' | 'Pending',
          lastActive: updated.updated_at ? new Date(updated.updated_at).toLocaleDateString() : 'Active'
        } : u));
      })
      .catch(err => console.error("failed to save user", err));
    } else {
      fetch(`${ADMIN_API}/resources/user`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
      .then(res => res.json())
      .then(created => {
        setUsers([...users, {
          id: created.id,
          name: created.name || created.email || 'Anonymous',
          email: created.email || '',
          role: (created.role === 'Admin' || created.role === 'admin' ? 'Admin' : 'User') as 'User' | 'Admin' | 'Moderator',
          status: (created.status === 'Inactive' || created.status === 'inactive' ? 'Inactive' : created.status === 'Pending' || created.status === 'pending' ? 'Pending' : 'Active') as 'Active' | 'Inactive' | 'Pending',
          lastActive: 'Just now'
        }]);
      })
      .catch(err => console.error("failed to create user", err));
    }
    setIsUserModalOpen(false);
    setEditingUser(null);
    setFormData({});
  };

  const handleDeleteUser = (user: User) => {
    if (!confirm(`Are you sure you want to PERMANENTLY delete user ${user.name}?`)) return;
    fetch(`${ADMIN_API}/resources/user/${encodeURIComponent(user.id)}`, {
      method: 'DELETE'
    })
    .then(res => {
      if (res.ok) {
        setUsers(users.filter(u => u.id !== user.id));
        if (selectedUserId === user.id) {
          setSelectedUserId(null);
        }
      }
    })
    .catch(err => console.error("failed to delete user", err));
  };

  const handleEditAdmin = (admin: AdminUser) => {
    setEditingAdmin(admin);
    setFormData(admin);
    setIsAdminModalOpen(true);
  };

  const saveAdmin = () => {
    const email = formData.email || '';
    const role = formData.tier || 'Superuser';
    const password = formData.password || 'adminpassword123';

    if (editingAdmin) {
      fetch(`${ADMIN_API}/resources/AdminUser/${editingAdmin.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, role })
      })
      .then(res => {
        if (!res.ok) throw new Error("Failed to update admin user");
        fetchAdminUsers();
      })
      .catch(err => console.error("failed to save admin", err));
    } else {
      fetch(`${ADMIN_API}/resources/AdminUser`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, role, password })
      })
      .then(res => {
        if (!res.ok) throw new Error("Failed to create admin user");
        fetchAdminUsers();
      })
      .catch(err => console.error("failed to create admin", err));
    }
    setIsAdminModalOpen(false);
    setEditingAdmin(null);
    setFormData({});
  };

  const handleEditFlag = (flag: FeatureFlag) => {
    setEditingFlag(flag);
    setFormData(flag);
    setIsFlagModalOpen(true);
  };

  const handleToggleFlag = (flag: FeatureFlag) => {
    fetch(`${ADMIN_API}/flags/${encodeURIComponent(flag.key)}/toggle`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: !flag.enabled })
    })
    .then(res => res.json())
    .then(updated => {
      setFlags(flags.map(f => f.key === flag.key ? { ...f, enabled: updated.enabled } : f));
    })
    .catch(err => console.error("failed to toggle flag", err));
  };

  const saveFlag = () => {
    if (editingFlag) {
      fetch(`${ADMIN_API}/flags/${encodeURIComponent(editingFlag.key)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData)
      })
      .then(res => res.json())
      .then(updated => {
        setFlags(flags.map(f => f.id === editingFlag.id ? { ...f, ...updated } : f));
      })
      .catch(err => console.error("failed to save flag", err));
    } else {
      fetch(`${ADMIN_API}/flags`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData)
      })
      .then(res => res.json())
      .then(newFlag => {
        setFlags([...flags, {
          id: newFlag.key,
          key: newFlag.key,
          description: newFlag.description || '',
          enabled: newFlag.enabled || false,
          environment: 'Production' as const,
          targeting: newFlag.targeting || { users: [], segments: [], rollout: 0 }
        }]);
      })
      .catch(err => console.error("failed to create flag", err));
    }
    setIsFlagModalOpen(false);
    setEditingFlag(null);
    setFormData({});
  };

  const saveTargetingChanges = () => {
    if (!activeFlag || !targetingEdit) return;
    const updatedFlag = {
      ...activeFlag,
      targeting: targetingEdit
    };
    fetch(`${ADMIN_API}/flags/${encodeURIComponent(activeFlag.key)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updatedFlag)
    })
    .then(res => {
      if (!res.ok) throw new Error("HTTP error " + res.status);
      return res.json();
    })
    .then(updated => {
      setFlags(flags.map(f => f.key === activeFlag.key ? { ...f, targeting: updated.targeting } : f));
      alert("Targeting rules saved successfully!");
    })
    .catch(err => {
      console.error("failed to save targeting rules", err);
      alert("Failed to save targeting rules: " + err.message);
    });
  };

  const handleAddUserKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && newUserKey.trim()) {
      e.preventDefault();
      if (targetingEdit && !targetingEdit.users.includes(newUserKey.trim())) {
        setTargetingEdit({
          ...targetingEdit,
          users: [...targetingEdit.users, newUserKey.trim()]
        });
      }
      setNewUserKey('');
    }
  };

  const handleToggleSegment = (segment: string) => {
    if (!targetingEdit) return;
    const exists = targetingEdit.segments.includes(segment);
    const updated = exists
      ? targetingEdit.segments.filter(s => s !== segment)
      : [...targetingEdit.segments, segment];
    setTargetingEdit({
      ...targetingEdit,
      segments: updated
    });
  };

  const fetchIncidents = () => {
    if (!providers.incident?.capabilities?.includes('read.list')) return;
    fetch(`${ADMIN_API}/incidents`)
      .then(res => {
        clearAuthIfUnauthorized(res, setIsAuthenticated);
        if (res.status === 401) throw new Error('Unauthorized');
        if (!res.ok) throw new Error("Failed to fetch incidents");
        return res.json();
      })
      .then(data => {
        if (Array.isArray(data)) {
          setIncidents(data);
        }
      })
      .catch(err => console.error("failed to fetch incidents", err));
  };

  const handleResolveIncident = (id: string) => {
    if (!providers.incident?.capabilities?.includes('resolve')) return;
    fetch(`${ADMIN_API}/incidents/${encodeURIComponent(id)}/resolve`, {
      method: 'POST'
    })
    .then(res => {
      if (res.ok) {
        setIncidents(prev => prev.filter(inc => inc.id !== id));
      } else {
        alert("Failed to resolve incident");
      }
    })
    .catch(err => {
      console.error("failed to resolve incident", err);
      alert("Error: " + err.message);
    });
  };

  const saveResource = () => {
    if (editingResource) {
      fetch(`${ADMIN_API}/resources/${encodeURIComponent(selectedCollectionId)}/${encodeURIComponent(editingResource.id)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData)
      })
      .then(res => res.json())
      .then(updated => {
        setCollections(collections.map(c => {
          if (c.id === selectedCollectionId) {
            return { ...c, data: c.data.map(r => r.id === editingResource.id ? updated : r) };
          }
          return c;
        }));
      })
      .catch(err => console.error("failed to update resource", err));
    } else {
      fetch(`${ADMIN_API}/resources/${encodeURIComponent(selectedCollectionId)}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData)
      })
      .then(res => res.json())
      .then(created => {
        setCollections(collections.map(c => {
          if (c.id === selectedCollectionId) {
            return { ...c, data: [...c.data, created] };
          }
          return c;
        }));
      })
      .catch(err => console.error("failed to create resource", err));
    }
    setIsResourceModalOpen(false);
    setEditingResource(null);
    setFormData({});
  };

  const handleSort = (field: string) => {
    setResourceQueryParams(prev => {
      let nextSort = `${field}_asc`;
      if (prev.sort === `${field}_asc`) {
        nextSort = `${field}_desc`;
      } else if (prev.sort === `${field}_desc`) {
        nextSort = '';
      }
      return {
        ...prev,
        page: 1,
        sort: nextSort
      };
    });
  };

  const handleDeleteResource = (rec: any) => {
    if (!confirm("Are you sure you want to PERMANENTLY delete this record?")) return;
    fetch(`${ADMIN_API}/resources/${encodeURIComponent(selectedCollectionId)}/${encodeURIComponent(rec.id)}`, {
      method: 'DELETE'
    })
    .then(res => {
      if (res.ok) {
        setCollections(collections.map(c => {
          if (c.id === selectedCollectionId) {
            return { ...c, data: c.data.filter((r: any) => r.id !== rec.id) };
          }
          return c;
        }));
      }
    })
    .catch(err => console.error("failed to delete record", err));
  };

  const handleCollectionAction = (act: any) => {
    if (act.confirm && !confirm(act.confirm)) return;

    if (act.builtin === 'csv_export') {
      window.open(`${ADMIN_API}/resources/${encodeURIComponent(selectedCollectionId)}/export.csv`, '_blank');
      return;
    }

    fetch(`${ADMIN_API}/resources/${encodeURIComponent(selectedCollectionId)}/collection/${encodeURIComponent(act.name)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    .then(res => {
      if (res.ok) {
        fetchCollectionData(selectedCollectionId);
        alert("Action executed successfully");
      } else {
        res.text().then(text => alert("Action failed: " + text));
      }
    })
    .catch(err => {
      console.error(err);
      alert("Error: " + err.message);
    });
  };

  const handleMemberAction = (act: any, recordId: string) => {
    if (act.confirm && !confirm(act.confirm)) return;

    fetch(`${ADMIN_API}/resources/${encodeURIComponent(selectedCollectionId)}/${encodeURIComponent(recordId)}/actions/${encodeURIComponent(act.name)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    .then(res => {
      if (res.ok) {
        fetchCollectionData(selectedCollectionId);
        alert("Action executed successfully");
      } else {
        res.text().then(text => alert("Action failed: " + text));
      }
    })
    .catch(err => {
      console.error(err);
      alert("Error: " + err.message);
    });
  };

  const handleBatchAction = (act: any) => {
    if (selectedRowIds.length === 0) {
      alert("No records selected");
      return;
    }
    if (act.confirm && !confirm(act.confirm)) return;

    fetch(`${ADMIN_API}/resources/${encodeURIComponent(selectedCollectionId)}/batch/${encodeURIComponent(act.name)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids: selectedRowIds })
    })
    .then(res => {
      if (res.ok) {
        setSelectedRowIds([]);
        fetchCollectionData(selectedCollectionId);
        alert("Batch action executed successfully");
      } else {
        res.text().then(text => alert("Batch action failed: " + text));
      }
    })
    .catch(err => {
      console.error(err);
      alert("Error: " + err.message);
    });
  };

  const reloadAdminConfig = React.useCallback(async () => {
    const res = await adminFetch(`${ADMIN_API}/config`);
    clearAuthIfUnauthorized(res, setIsAuthenticated);
    if (!res.ok) return;
    const config = await res.json();
    setAdminConfig(config);
    if (config?.resources) {
      const allResources: any[] = [];
      config.resources.forEach((r: any) => {
        if (r.enabled === false) return;
        allResources.push({
          id: r.resource,
          name: r.resource,
          displayName: r.menu?.label || r.resource,
          fields: r.fields || [],
          data: [],
          pinnedToMenu: !!r.menu?.pin,
          spec: r,
        });
      });
      allResources.sort((a: any, b: any) => a.name.localeCompare(b.name));
      setCollections(allResources);
    }
  }, [ADMIN_API]);

  const togglePinning = async (id: string) => {
    const coll = collections.find((c) => c.id === id);
    const next = !coll?.pinnedToMenu;
    const res = await adminFetch(`${ADMIN_API}/nav-preferences`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ kind: 'resource', name: id, pin: next }),
    });
    clearAuthIfUnauthorized(res, setIsAuthenticated);
    if (!res.ok) return;
    await reloadAdminConfig();
  };

  const selectedFeatureNavPage = selectedScreenName
    ? `screen:${selectedScreenName}`
    : selectedClusterName
      ? `feature:${selectedClusterName}`
      : null;

  const isSelectedFeaturePinned = useMemo(() => {
    if (!selectedFeatureNavPage) return false;
    return menuContainsPage(adminConfig?.site?.menu ?? [], selectedFeatureNavPage);
  }, [adminConfig?.site?.menu, selectedFeatureNavPage]);

  const toggleFeaturePinning = async () => {
    if (!selectedFeatureNavPage) return;
    const res = await adminFetch(`${ADMIN_API}/nav-preferences`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        kind: 'feature',
        name: selectedFeatureNavPage,
        pin: !isSelectedFeaturePinned,
      }),
    });
    clearAuthIfUnauthorized(res, setIsAuthenticated);
    if (!res.ok) return;
    await reloadAdminConfig();
  };

  if (!authChecked) {
    return (
      <div className="min-h-screen bg-[#07090e] flex items-center justify-center font-display text-white">
        <div className="text-sm text-[#9BA3AF]">Loading Admin Console...</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <div className="min-h-screen bg-[#07090e] flex items-center justify-center relative overflow-hidden font-display text-white">
        {/* Glow circles */}
        <div className="absolute top-[-20%] left-[-10%] w-[600px] h-[600px] rounded-full bg-blue-600/10 blur-[150px]" />
        <div className="absolute bottom-[-20%] right-[-10%] w-[600px] h-[600px] rounded-full bg-blue-500/10 blur-[150px]" />
        
        <div className="w-full max-w-md bg-[#0F1216]/65 backdrop-blur-2xl border border-white/5 rounded-3xl p-10 shadow-2xl relative z-10 font-sans">
          <div className="flex flex-col items-center mb-8">
            <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-blue-600 to-blue-400 flex items-center justify-center shadow-lg shadow-blue-500/20 mb-4 border border-white/10">
              <span className="text-2xl font-bold font-display tracking-tight text-white">B</span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-white font-display">BFFX Console</h1>
            <p className="text-xs text-[#8c90a1] mt-1 font-mono uppercase tracking-widest">Sign in to control room</p>
          </div>

          <form onSubmit={handleLogin} className="space-y-6">
            <div>
              <label className="block text-[10px] font-bold text-[#8c90a1] uppercase tracking-widest mb-2 font-mono">Username</label>
              <input 
                type="text" 
                required 
                onChange={(e) => setFormData({ ...formData, username: e.target.value })} 
                className="w-full bg-[#161B22] border border-border-subtle rounded-xl px-4 py-3 text-sm text-white focus:border-blue-500/50 outline-none transition-all"
                placeholder="admin@example.com"
              />
            </div>

            <div>
              <label className="block text-[10px] font-bold text-[#8c90a1] uppercase tracking-widest mb-2 font-mono">Password</label>
              <input 
                type="password" 
                required 
                onChange={(e) => setFormData({ ...formData, password: e.target.value })} 
                className="w-full bg-[#161B22] border border-border-subtle rounded-xl px-4 py-3 text-sm text-white focus:border-blue-500/50 outline-none transition-all"
                placeholder="••••••••"
              />
            </div>

            {loginError && (
              <div className="text-xs text-red-400 bg-red-400/10 border border-red-400/20 rounded-xl p-3 text-center font-bold">
                {loginError}
              </div>
            )}

            <button 
              type="submit" 
              className="w-full py-3 bg-blue-500 hover:bg-blue-600 text-black font-bold rounded-xl text-sm transition-all shadow-lg shadow-blue-500/10 active:scale-[0.98]"
            >
              Access Dashboard
            </button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen">
      {/* SideNavBar */}
      <aside className="fixed left-0 top-0 h-screen w-72 border-r border-border-subtle bg-surface-dim flex flex-col z-50">
        <div className="p-6 border-b border-border-subtle flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-surface-card border border-border-subtle flex items-center justify-center shadow-[0_0_20px_rgba(13,110,253,0.15)] text-blue-400">
            <Server size={24} />
          </div>
          <div>
            <h1 className="font-display text-xl font-bold text-text-main leading-tight">{adminConfig?.site?.title || 'Obsidian'}</h1>
            <p className="font-sans text-xs text-[#9BA3AF]">Backend Console</p>
          </div>
        </div>

        <nav className="flex-1 py-4 overflow-y-auto">
          <AdminNav
            menu={adminConfig?.site?.menu ?? []}
            features={adminConfig?.site?.features}
            providers={providers}
            activePage={activePage}
            selectedCollectionId={selectedCollectionId}
            selectedScreenName={selectedScreenName}
            selectedClusterName={selectedClusterName}
            onNavigate={(page, resourceId) => {
              if (page.startsWith('screen:')) {
                setSelectedScreenName(page.slice('screen:'.length));
                setSelectedClusterName(null);
                setActivePage('app-features');
                return;
              }
              if (page.startsWith('feature:')) {
                setSelectedClusterName(page.slice('feature:'.length));
                setSelectedScreenName(null);
                setActivePage('app-features');
                return;
              }
              if (resourceId) {
                setSelectedCollectionId(resourceId);
              }
              if (page === 'app-features') {
                setSelectedScreenName(null);
                setSelectedClusterName(null);
              }
              setActivePage(page as Page);
            }}
            SidebarItem={SidebarItem}
          />
        </nav>

        <div className="p-6 border-t border-border-subtle flex items-center gap-3">
          <img 
            src="https://images.unsplash.com/photo-1544005313-94ddf0286df2?auto=format&fit=crop&q=80&w=80" 
            className="w-10 h-10 rounded-full border border-border-subtle" 
            alt="Profile" 
          />
          <div className="flex-1 overflow-hidden">
            <p className="font-sans text-sm font-medium text-text-main truncate">Admin User</p>
            <p className="font-display text-[10px] uppercase font-bold text-[#9BA3AF] truncate tracking-widest">System Root</p>
          </div>
          <LogOut onClick={handleLogout} size={18} className="text-[#8c90a1] hover:text-red-400 transition-colors cursor-pointer" />
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="ml-72 flex-1 flex flex-col min-h-screen">
        {/* TopNavBar */}
        <header className="fixed top-0 right-0 w-[calc(100%-18rem)] h-16 border-b border-border-subtle bg-background/80 backdrop-blur-md flex justify-between items-center px-8 z-40">
          <div className="flex items-center gap-4">
            <button className="text-[#8c90a1] hover:text-text-main lg:hidden">
              <Database size={20} />
            </button>
            <span className="font-display font-medium text-text-main capitalize">{activePage.replace('-', ' ')}</span>
            <span className="text-[#8c90a1] text-[10px] font-bold uppercase">/</span>
            <span className="text-blue-400 font-bold border-b-2 border-blue-400 text-[10px] uppercase tracking-widest pb-0.5">Live</span>
          </div>

          <div className="flex items-center gap-6">
            <div className="relative">
              <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-[#8c90a1]" />
              <input 
                type="text" 
                placeholder="Global Search..."
                className="bg-surface-card border border-border-subtle rounded-lg pl-10 pr-4 py-2 text-sm focus:outline-none focus:border-blue-400 focus:ring-1 focus:ring-blue-400/50 transition-all font-sans text-text-main w-64 placeholder-[#8c90a1]"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <button 
              onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
              className="text-[#8c90a1] hover:text-blue-400 transition-all p-2 rounded-lg hover:bg-surface-card"
              title={theme === 'dark' ? 'Switch to Light Mode' : 'Switch to Dark Mode'}
            >
              {theme === 'dark' ? <Sun size={20} /> : <Moon size={20} />}
            </button>
            <button
              type="button"
              onClick={() => setActivePage('alerts')}
              className={cn(
                'transition-colors p-2 rounded-lg hover:bg-surface-card',
                activePage === 'alerts' ? 'text-blue-400' : 'text-[#8c90a1] hover:text-blue-400',
              )}
              title="Alerts"
            >
              <Bell size={20} />
            </button>
            <button
              type="button"
              onClick={() => setActivePage('system-settings')}
              className={cn(
                'transition-colors p-2 rounded-lg hover:bg-surface-card',
                activePage === 'system-settings' ? 'text-blue-400' : 'text-[#8c90a1] hover:text-blue-400',
              )}
              title="System settings"
            >
              <Settings size={20} />
            </button>
          </div>
        </header>

        {/* Page Content Rendering */}
        <div className="mt-16 p-8 flex-1">
          <AnimatePresence mode="wait">
            {activePage === 'dashboard' && (
              <motion.div key="dashboard" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
                <PageHeader 
                  title="System Intelligence" 
                  description="Real-time monitoring and advanced infrastructure insights."
                  actions={
                    <div className="flex gap-2">
                       <button className="px-3 py-1.5 bg-surface-dim border border-border-subtle rounded-lg text-[10px] font-bold text-[#8c90a1] uppercase tracking-widest hover:text-white transition-all">Today</button>
                       <button className="px-4 py-2 bg-blue-500 text-black font-bold rounded-lg text-sm flex items-center gap-2 hover:bg-blue-400 transition-colors shadow-lg shadow-blue-500/20">
                         <Download size={16} /> Data Export
                       </button>
                    </div>
                  }
                />
                {/* Dashboard Widgets */}
                {dashboardWidgets && dashboardWidgets.length > 0 ? (
                  <>
                    {/* Metrics Section */}
                    {dashboardWidgets.some(w => w.type === 'metric') && (
                      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
                        {dashboardWidgets.filter(w => w.type === 'metric').map((w, idx) => (
                          <StatCard 
                            key={idx} 
                            title={w.title} 
                            value={w.data !== null && w.data !== undefined ? (typeof w.data === 'number' ? w.data.toLocaleString() : String(w.data)) : '0'} 
                            trend="up" 
                            trendValue="Live" 
                            icon={Activity} 
                          />
                        ))}
                      </div>
                    )}

                    {/* Charts & Tables Section */}
                    {dashboardWidgets.some(w => w.type === 'chart' || w.type === 'table') && (
                      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
                        {dashboardWidgets.filter(w => w.type === 'chart' || w.type === 'table').map((w, idx) => (
                          <div key={idx} className="flex flex-col">
                            {w.type === 'chart' ? (
                              <div className="bg-surface-card border border-border-subtle rounded-2xl p-6 h-[320px] flex flex-col shadow-2xl">
                                <div className="flex justify-between items-center mb-6">
                                   <h4 className="font-display text-[10px] font-bold text-[#8c90a1] uppercase tracking-[0.2em]">{w.title}</h4>
                                   <Activity size={14} className="text-blue-400" />
                                </div>
                                <div className="flex-1 min-h-0">
                                  <ResponsiveContainer width="100%" height="100%">
                                     <AreaChart data={w.data || []}>
                                        <defs>
                                          <linearGradient id={`color-${idx}`} x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4}/>
                                            <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
                                          </linearGradient>
                                        </defs>
                                        <CartesianGrid strokeDasharray="3 3" stroke="#2D343C" vertical={false} strokeOpacity={0.3} />
                                        <XAxis dataKey="time" stroke="#424655" fontSize={10} axisLine={false} tickLine={false} />
                                        <YAxis stroke="#424655" fontSize={10} axisLine={false} tickLine={false} />
                                        <Tooltip contentStyle={{ backgroundColor: '#111416', border: '1px solid #2D343C', borderRadius: '12px' }} />
                                        <Area type="monotone" dataKey="value" stroke="#3b82f6" strokeWidth={3} fill={`url(#color-${idx})`} />
                                     </AreaChart>
                                  </ResponsiveContainer>
                                </div>
                              </div>
                            ) : (
                              <div className="bg-surface-card border border-border-subtle rounded-2xl p-6 flex flex-col shadow-2xl min-h-[320px]">
                                <h3 className="font-display text-[10px] font-bold text-[#8c90a1] uppercase tracking-[0.2em] mb-6">{w.title}</h3>
                                <div className="overflow-x-auto flex-1">
                                  <table className="w-full text-left text-xs border-collapse">
                                    <thead>
                                      <tr className="border-b border-border-subtle text-[10px] uppercase tracking-wider text-[#8c90a1]">
                                        {w.columns && w.columns.map((col: string) => (
                                          <th key={col} className="pb-3 pt-2 font-bold">{col}</th>
                                        ))}
                                      </tr>
                                    </thead>
                                    <tbody>
                                      {w.data && Array.isArray(w.data) && w.data.map((row: any, i: number) => (
                                        <tr key={row.id || i} className="border-b border-border-subtle/50 hover:bg-white/5 transition-colors">
                                          {w.columns && w.columns.map((col: string) => (
                                            <td key={col} className="py-3 font-mono font-medium text-text-main">
                                              {typeof row[col] === 'object' ? JSON.stringify(row[col]) : String(row[col] ?? '')}
                                            </td>
                                          ))}
                                        </tr>
                                      ))}
                                    </tbody>
                                  </table>
                                </div>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </>
                ) : (
                  <>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
                      <StatCard title="API Volumetric" value={metricsSummary.total_requests?.toLocaleString() || '0'} trend="up" trendValue="Live" icon={RefreshCw} />
                      <StatCard title="P95 Latency" value={`${metricsSummary.avg_latency_ms || 0}ms`} trend="down" trendValue="Live" icon={Zap} />
                      <StatCard title="Active Sessions" value={metricsSummary.active_sessions?.toLocaleString() || '0'} trend="up" trendValue="Live" icon={Users} />
                      <StatCard title="System Health" value={`${((1.0 - (metricsSummary.error_rate || 0)) * 100).toFixed(2)}%`} trend="up" trendValue="SLA Met" icon={ShieldCheck} />
                    </div>

                    <div className="grid grid-cols-1 lg:grid-cols-6 gap-6 mb-8">
                      {/* Ingress Traffic - Main */}
                      <div className="lg:col-span-4 bg-surface-card border border-border-subtle rounded-2xl p-8 flex flex-col shadow-2xl relative overflow-hidden group">
                        <div className="absolute top-0 right-0 p-8 opacity-10 group-hover:opacity-20 transition-opacity">
                          <BarChart2 size={120} />
                        </div>
                        <div className="flex justify-between items-start mb-8 relative z-10">
                          <div>
                            <h3 className="font-display text-xl font-bold text-white tracking-tight">Ingress Traffic</h3>
                            <p className="text-[10px] text-[#8c90a1] font-bold uppercase tracking-[0.2em] mt-1.5 flex items-center gap-2">
                              <span className="w-2 h-2 rounded-full bg-blue-500 animate-pulse" />
                              Live Stream: Global Request Flow
                            </p>
                          </div>
                          <div className="flex items-center gap-4 bg-background/50 backdrop-blur-sm border border-border-subtle rounded-lg px-4 py-2">
                             <div className="flex items-center gap-2">
                                <div className="w-2 h-2 rounded-full bg-blue-500" />
                                <span className="text-xs font-mono text-white">4.2k req/s</span>
                             </div>
                          </div>
                        </div>
                        <div className="flex-1 min-h-[300px]">
                          <ResponsiveContainer width="100%" height="100%">
                            <AreaChart data={perfData}>
                              <defs>
                                <linearGradient id="gBlue" x1="0" y1="0" x2="0" y2="1">
                                  <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4}/>
                                  <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
                                </linearGradient>
                              </defs>
                              <CartesianGrid strokeDasharray="3 3" stroke="#2D343C" vertical={false} strokeOpacity={0.5} />
                              <XAxis dataKey="time" stroke="#424655" fontSize={10} axisLine={false} tickLine={false} tickMargin={15} />
                              <YAxis stroke="#424655" fontSize={10} axisLine={false} tickLine={false} />
                              <Tooltip 
                                contentStyle={{ backgroundColor: 'var(--surface-card)', border: '1px solid var(--border-subtle)', borderRadius: '16px', padding: '12px', boxShadow: '0 20px 25px -5px rgba(0,0,0,0.5)' }}
                                itemStyle={{ fontSize: '12px', color: 'var(--text-main)', fontWeight: 'bold' }}
                                labelStyle={{ fontSize: '10px', color: '#8c90a1', textTransform: 'uppercase', letterSpacing: '0.1em', marginBottom: '8px' }}
                                cursor={{ stroke: '#3b82f6', strokeWidth: 2 }}
                              />
                              <Area 
                                type="monotone" 
                                dataKey="traffic" 
                                stroke="#3b82f6" 
                                strokeWidth={4} 
                                fill="url(#gBlue)" 
                                animationDuration={2000}
                                activeDot={{ r: 6, fill: '#fff', stroke: '#3b82f6', strokeWidth: 3 }}
                              />
                            </AreaChart>
                          </ResponsiveContainer>
                        </div>
                      </div>

                      {/* Regional Status */}
                      <div className="lg:col-span-2 bg-surface-card border border-border-subtle rounded-2xl p-8 flex flex-col shadow-2xl">
                         <h3 className="font-display text-lg font-bold text-white mb-8">Node Distribution</h3>
                         <AnalyticsWidgetPlaceholder widget="regional" analytics={providers.analytics} icon={Server} />
                      </div>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
                       {/* Latency Chart */}
                       <div className="bg-surface-card border border-border-subtle rounded-2xl p-6 h-[320px] flex flex-col">
                          <div className="flex justify-between items-center mb-6">
                             <h4 className="font-display text-[10px] font-bold text-[#8c90a1] uppercase tracking-[0.2em]">Edge Latency (ms)</h4>
                             <Zap size={14} className="text-blue-400" />
                          </div>
                          <div className="flex-1">
                            <ResponsiveContainer width="100%" height="100%">
                               <LineChart data={perfData}>
                                  <CartesianGrid strokeDasharray="3 3" stroke="#2D343C" vertical={false} strokeOpacity={0.3} />
                                  <XAxis dataKey="time" hide />
                                  <YAxis domain={['auto', 'auto']} hide />
                                  <Tooltip contentStyle={{ backgroundColor: '#111416', border: '1px solid #2D343C', borderRadius: '12px' }} />
                                  <Line type="monotone" dataKey="latency" stroke="#3b82f6" strokeWidth={3} dot={false} animationDuration={2000} />
                               </LineChart>
                            </ResponsiveContainer>
                          </div>
                       </div>

                       {/* Error Rate Breakdown */}
                       <div className="bg-surface-card border border-border-subtle rounded-2xl p-6 h-[320px] flex flex-col">
                          <div className="flex justify-between items-center mb-6">
                             <h4 className="font-display text-[10px] font-bold text-[#8c90a1] uppercase tracking-[0.2em]">Response Categories</h4>
                             <ShieldCheck size={14} className="text-green-400" />
                          </div>
                          <div className="flex-1">
                            {(() => {
                              const latestSample = perfData[perfData.length - 1] || { traffic: 100, errors: 0 };
                              const traffic = latestSample.traffic || 0;
                              const errors = latestSample.errors || 0;
                              const okRequests = Math.max(0, traffic - errors);
                              const errorRequests = errors;
                              const dynamicResponseData = [
                                { code: '2xx', count: okRequests, color: '#3b82f6' },
                                { code: '4xx', count: 0, color: '#f59e0b' },
                                { code: '5xx', count: errorRequests, color: '#ef4444' }
                              ];
                              return (
                                <ResponsiveContainer width="100%" height="100%">
                                   <BarChart data={dynamicResponseData} layout="vertical">
                                      <XAxis type="number" hide />
                                      <YAxis dataKey="code" type="category" axisLine={false} tickLine={false} stroke="#8c90a1" fontSize={10} width={40} />
                                      <Tooltip cursor={{ fill: 'rgba(255,255,255,0.02)' }} contentStyle={{ backgroundColor: '#111416', border: '1px solid #2D343C', borderRadius: '12px' }} />
                                      <Bar dataKey="count" radius={[0, 4, 4, 0]} barSize={20}>
                                         {dynamicResponseData.map((entry, index) => (
                                            <Cell key={`cell-${index}`} fill={entry.color} />
                                         ))}
                                      </Bar>
                                   </BarChart>
                                </ResponsiveContainer>
                              );
                            })()}
                          </div>
                       </div>

                       {/* Push Performance */}
                       <div className="bg-surface-card border border-border-subtle rounded-2xl p-6 h-[320px] flex flex-col">
                          <div className="flex justify-between items-center mb-6">
                             <h4 className="font-display text-[10px] font-bold text-[#8c90a1] uppercase tracking-[0.2em]">Push Propagation</h4>
                             <Smartphone size={14} className="text-purple-400" />
                          </div>
                          <div className="flex-1">
                            <AnalyticsWidgetPlaceholder widget="push" analytics={providers.analytics} icon={Smartphone} />
                          </div>
                       </div>

                       {/* Resource Efficiency */}
                       <div className="bg-surface-card border border-border-subtle rounded-2xl p-8 h-[340px] flex flex-col lg:col-span-2">
                           <div className="flex justify-between items-center mb-8">
                              <div>
                                 <h4 className="font-display text-sm font-bold text-white tracking-tight">Compute Infrastructure</h4>
                                 <p className="text-[10px] text-[#424655] font-bold uppercase tracking-widest mt-1">Cluster Load Metrics</p>
                              </div>
                              <div className="flex gap-4">
                                 <div className="flex items-center gap-2">
                                    <div className="w-3 h-3 rounded bg-blue-500" />
                                    <span className="text-[10px] font-bold text-[#8c90a1] uppercase tracking-widest">CPU Core</span>
                                 </div>
                                 <div className="flex items-center gap-2">
                                    <div className="w-3 h-3 rounded bg-yellow-500" />
                                    <span className="text-[10px] font-bold text-[#8c90a1] uppercase tracking-widest">Memory</span>
                                 </div>
                              </div>
                           </div>
                           <div className="flex-1">
                              <ResponsiveContainer width="100%" height="100%">
                                 <LineChart data={perfData}>
                                    <CartesianGrid strokeDasharray="3 3" stroke="#2D343C" vertical={false} strokeOpacity={0.2} />
                                    <XAxis dataKey="time" stroke="#424655" fontSize={10} axisLine={false} tickLine={false} />
                                    <YAxis hide domain={[0, 100]} />
                                    <Tooltip contentStyle={{ backgroundColor: '#111416', border: '1px solid #2D343C', borderRadius: '16px' }} />
                                    <Line type="stepAfter" dataKey="cpu" stroke="#3b82f6" strokeWidth={3} dot={{ r: 4, fill: '#3b82f6', strokeWidth: 0 }} activeDot={{ r: 6 }} />
                                    <Line type="stepAfter" dataKey="mem" stroke="#f59e0b" strokeWidth={3} dot={{ r: 4, fill: '#f59e0b', strokeWidth: 0 }} activeDot={{ r: 6 }} />
                                 </LineChart>
                              </ResponsiveContainer>
                           </div>
                       </div>

                       {/* Concurrency / Active Sessions */}
                       <div className="bg-surface-card border border-border-subtle rounded-2xl p-8 h-[340px] flex flex-col">
                           <div className="mb-8">
                              <h4 className="font-display text-sm font-bold text-white tracking-tight">Session Concurrency</h4>
                              <p className="text-[10px] text-[#424655] font-bold uppercase tracking-widest mt-1">Unique Global Handshakes</p>
                           </div>
                           <div className="flex-1">
                              <ResponsiveContainer width="100%" height="100%">
                                 <BarChart data={perfData}>
                                    <XAxis dataKey="time" hide />
                                    <YAxis hide />
                                    <Tooltip cursor={{ fill: 'rgba(255,255,255,0.03)' }} contentStyle={{ backgroundColor: '#111416', border: '1px solid #2D343C', borderRadius: '12px' }} />
                                    <Bar dataKey="users" fill="#3b82f6" radius={[6, 6, 0, 0]} barSize={30} />
                                 </BarChart>
                              </ResponsiveContainer>
                           </div>
                       </div>
                    </div>
                  </>
                )}
              </motion.div>
            )}

            {activePage === 'users' && (
              <motion.div key="users" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
                {!selectedUserId ? (
                  <>
                    <PageHeader 
                      title="Member Directory" 
                      description="Manage platform users and their subscription states."
                      actions={
                        <button 
                          onClick={() => { setEditingUser(null); setFormData({}); setIsUserModalOpen(true); }}
                          className="px-4 py-2 bg-blue-500 text-black font-bold rounded-lg text-sm flex items-center gap-2 hover:bg-blue-400 transition-colors"
                        >
                          <Plus size={18} /> New User
                        </button>
                      }
                    />
                    
                    <div className="bg-surface-card border border-border-subtle rounded-xl overflow-hidden shadow-xl">
                      <div className="p-4 bg-surface-dim border-b border-border-subtle flex justify-between items-center">
                        <div className="flex gap-2">
                           <button className="p-2 text-[#8c90a1] hover:text-white border border-border-subtle rounded-lg bg-surface-card"><Filter size={16} /></button>
                           <div className="relative">
                              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[#8c90a1]" />
                              <input 
                                type="text" 
                                placeholder="Filter members..." 
                                className="bg-background border border-border-subtle rounded-lg pl-9 pr-4 py-1.5 text-xs text-white w-48 focus:border-blue-400 outline-none"
                                value={search}
                                onChange={(e) => setSearch(e.target.value)}
                              />
                           </div>
                        </div>
                      </div>
                      <div className="overflow-x-auto">
                        <table className="w-full text-left">
                          <thead className="bg-[#111416] border-b border-border-subtle font-display text-[10px] font-bold text-[#9BA3AF] uppercase tracking-widest">
                            <tr>
                              <th className="py-4 px-6">Member</th>
                              <th className="py-4 px-6">Role</th>
                              <th className="py-4 px-6">Status</th>
                              <th className="py-4 px-6">Last Active</th>
                              <th className="py-4 px-6 text-right">Actions</th>
                            </tr>
                          </thead>
                          <tbody className="font-sans text-sm text-white divide-y divide-border-subtle">
                            {filteredUsers.map(user => (
                              <tr key={user.id} onClick={() => setSelectedUserId(user.id)} className="hover:bg-surface-dim transition-colors cursor-pointer group">
                                <td className="py-4 px-6">
                                  <div className="flex items-center gap-3">
                                    <div className="w-8 h-8 rounded-full bg-blue-400/10 flex items-center justify-center text-blue-400 font-bold text-xs">{user.name.charAt(0)}</div>
                                    <div>
                                      <p className="font-medium">{user.name}</p>
                                      <p className="text-xs text-[#9BA3AF]">{user.email}</p>
                                    </div>
                                  </div>
                                </td>
                                <td className="py-4 px-6">
                                   <span className="font-mono text-xs text-[#8c90a1]">{user.role}</span>
                                </td>
                                <td className="py-4 px-6">
                                  <span className={cn(
                                    "px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase border",
                                    user.status === 'Active' ? "bg-green-500/10 text-green-400 border-green-500/20" : 
                                    user.status === 'Inactive' ? "bg-red-500/10 text-red-400 border-red-500/20" : "bg-orange-500/10 text-orange-400 border-orange-500/20"
                                  )}>
                                    {user.status}
                                  </span>
                                </td>
                                <td className="py-4 px-6 text-[#9BA3AF] font-mono text-xs">{user.lastActive}</td>
                                <td className="py-4 px-6 text-right">
                                   <div className="flex justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                                      <button onClick={(e) => { e.stopPropagation(); handleEditUser(user); }} className="p-1.5 hover:text-blue-400 hover:bg-blue-400/10 rounded transition-all"><Edit2 size={14} /></button>
                                      <button onClick={(e) => { e.stopPropagation(); handleDeleteUser(user); }} className="p-1.5 hover:text-red-400 hover:bg-red-400/10 rounded transition-all"><Trash2 size={14} /></button>
                                      <ChevronRight size={14} className="text-[#424655]" />
                                   </div>
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  </>
                ) : (
                  <div className="space-y-6">
                    <button 
                      onClick={() => setSelectedUserId(null)}
                      className="flex items-center gap-2 text-[#8c90a1] hover:text-white transition-all group mb-4"
                    >
                      <ChevronRight size={18} className="rotate-180 group-hover:-translate-x-1 transition-transform" />
                      <span className="font-display font-bold text-xs tracking-widest">BACK TO DIRECTORY</span>
                    </button>

                    {(() => {
                      const user = users.find(u => u.id === selectedUserId);
                      const detail = userDetail || { user: user, devices: [], roles: [] };

                      // No real "orders" endpoint yet; keep the section as an empty-state placeholder.
                      const userOrders: Order[] = [];

                      const cleanUser = {
                        name: detail.user?.name || detail.user?.email || user?.name || 'Anonymous',
                        email: detail.user?.email || user?.email || '',
                        role: detail.roles && detail.roles.length > 0 
                          ? detail.roles.map((r: any) => r.role_id || r.role).join(', ') 
                          : (detail.user?.role || user?.role || 'User'),
                        status: detail.user?.status || user?.status || 'Active',
                        location: detail.user?.location || 'Location Pending',
                        phone: detail.user?.phone || 'No phone record',
                        joinDate: detail.user?.created_at ? new Date(detail.user.created_at).toLocaleDateString() : 'N/A',
                        subscription: detail.user?.subscription || 'Free Tier',
                        bio: detail.user?.bio || 'The system awaits a user-defined biography for this profile.',
                        lastActive: detail.user?.updated_at ? new Date(detail.user.updated_at).toLocaleDateString() : 'Active',
                        devices: (detail.devices || []).map((d: any) => ({
                          type: d.platform === 'ios' || d.platform === 'iOS' ? 'iOS' : d.platform === 'android' || d.platform === 'Android' ? 'Android' : 'Web',
                          name: d.device_name || d.name || 'Unknown Device',
                          lastUsed: d.updated_at ? new Date(d.updated_at).toLocaleDateString() : 'Active'
                        }))
                      };
                      
                      return (
                        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
                          <div className="lg:col-span-1 space-y-6">
                            <div className="bg-surface-card border border-border-subtle rounded-3xl p-8 flex flex-col items-center text-center shadow-2xl relative overflow-hidden">
                               <div className="absolute top-0 left-0 w-full h-24 bg-gradient-to-br from-blue-600/20 to-transparent" />
                               <div className="w-24 h-24 rounded-3xl bg-gradient-to-br from-blue-600 to-blue-400 border border-white/20 shadow-2xl flex items-center justify-center text-4xl font-bold text-white mb-6 relative z-10">
                                  {cleanUser.name.charAt(0)}
                               </div>
                               <h2 className="font-display text-2xl font-bold text-white leading-tight z-10">{cleanUser.name}</h2>
                               <p className="text-blue-400 font-mono text-sm mt-1 z-10">{cleanUser.email}</p>
                               <div className="mt-6 flex flex-wrap justify-center gap-2 z-10">
                                  <span className="px-3 py-1 bg-blue-400/10 text-blue-400 border border-blue-400/20 rounded-full text-[10px] font-bold uppercase tracking-widest">{cleanUser.role}</span>
                                  <span className={cn("px-3 py-1 border rounded-full text-[10px] font-bold uppercase tracking-widest", 
                                    cleanUser.status === 'Active' ? "border-blue-400/20 bg-blue-400/10 text-blue-400" : "border-border-subtle text-[#8c90a1]"
                                  )}>{cleanUser.status}</span>
                               </div>
                            </div>

                            <div className="bg-surface-card border border-border-subtle rounded-2xl p-6 space-y-4">
                               <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-4">Core Metadata</p>
                               <div className="flex items-center gap-3 text-sm">
                                  <MapPin size={16} className="text-[#8c90a1]" />
                                  <span className="text-white">{cleanUser.location}</span>
                               </div>
                               <div className="flex items-center gap-3 text-sm">
                                  <Phone size={16} className="text-[#8c90a1]" />
                                  <span className="text-white">{cleanUser.phone}</span>
                               </div>
                               <div className="flex items-center gap-3 text-sm">
                                  <Calendar size={16} className="text-[#8c90a1]" />
                                  <span className="text-[#8c90a1]">Registered: <span className="text-white">{cleanUser.joinDate}</span></span>
                                </div>
                               <div className="flex items-center gap-3 text-sm">
                                  <CreditCard size={16} className="text-[#8c90a1]" />
                                  <span className="text-[#8c90a1]">Subscription: <span className="text-blue-400 font-bold uppercase tracking-tighter">{cleanUser.subscription}</span></span>
                               </div>
                            </div>

                            <div className="bg-blue-500/5 border border-blue-400/10 rounded-2xl p-6">
                               <div className="flex items-center gap-3 text-blue-400 mb-3">
                                  <History size={18} />
                                  <h4 className="font-display font-bold text-sm tracking-tight">Recent Activity</h4>
                               </div>
                               <p className="text-xs text-[#8c90a1] leading-relaxed">Last identified session was <span className="text-white font-bold">{cleanUser.lastActive}</span> via secure gateway.</p>
                            </div>
                          </div>

                          <div className="lg:col-span-2 space-y-8">
                             <section className="bg-surface-card border border-border-subtle rounded-2xl p-8">
                                <div className="flex items-center gap-3 mb-6">
                                   <UserIcon size={20} className="text-blue-400" />
                                   <h3 className="font-display text-lg font-semibold text-white">Professional Biography</h3>
                                </div>
                                <p className="text-sm text-[#9BA3AF] leading-relaxed italic">"{cleanUser.bio}"</p>
                             </section>

                             <section className="bg-surface-card border border-border-subtle rounded-2xl overflow-hidden shadow-2xl">
                                <div className="p-6 border-b border-border-subtle bg-surface-dim flex justify-between items-center">
                                   <h3 className="font-display text-lg font-semibold text-white flex items-center gap-3">
                                      <Smartphone size={18} className="text-blue-400" /> Authenticated Devices
                                   </h3>
                                   <span className="text-[10px] font-mono text-[#8c90a1] uppercase tracking-widest">{cleanUser.devices.length} Assets</span>
                                </div>
                                <div className="divide-y divide-border-subtle/50">
                                   {cleanUser.devices.map((dev, i) => (
                                      <div key={i} className="p-4 flex items-center justify-between hover:bg-surface-dim transition-colors">
                                         <div className="flex items-center gap-4">
                                            <div className="w-10 h-10 rounded-xl bg-background border border-border-subtle flex items-center justify-center text-blue-400">
                                              {dev.type === 'iOS' ? <Smartphone size={20} /> : dev.type === 'Android' ? <Smartphone size={20} /> : <Laptop size={20} />}
                                            </div>
                                            <div>
                                               <p className="text-sm font-bold text-white">{dev.name}</p>
                                               <p className="text-[10px] font-mono text-[#424655] uppercase tracking-widest">{dev.type} Hardware</p>
                                            </div>
                                         </div>
                                         <div className="text-right">
                                            <p className="text-xs text-[#8c90a1]">{dev.lastUsed}</p>
                                            <div className="flex items-center justify-end gap-1.5 mt-1">
                                               <div className="w-1 h-1 rounded-full bg-blue-500 animate-pulse" />
                                               <span className="text-[10px] text-blue-400 font-bold uppercase">Linked</span>
                                            </div>
                                         </div>
                                      </div>
                                   ))}
                                   {!cleanUser.devices.length && (
                                     <div className="p-12 text-center text-[#424655] italic text-sm">No authorized hardware detected.</div>
                                   )}
                                </div>
                             </section>

                             <section className="bg-surface-card border border-border-subtle rounded-2xl overflow-hidden shadow-2xl">
                                <div className="p-6 border-b border-border-subtle bg-surface-dim flex justify-between items-center">
                                   <h3 className="font-display text-lg font-semibold text-white flex items-center gap-3">
                                      <Activity size={18} className="text-blue-400" /> Audit Activity
                                   </h3>
                                   <span className="text-[10px] font-mono text-[#8c90a1] uppercase tracking-widest">Security Log</span>
                                </div>
                                <div className="divide-y divide-border-subtle/50">
                                   {(() => {
                                      const auditsForUser = auditLogs.filter(a =>
                                         (a.actor_id && a.actor_id === cleanUser.email) ||
                                         (a.actor_id && a.actor_id === selectedUserId) ||
                                         (a.resource_id && a.resource_id === selectedUserId)
                                      );
                                      
                                      if (auditsForUser.length === 0) {
                                         return <div className="p-8 text-center text-[#424655] italic text-sm">No recent security events recorded.</div>;
                                      }

                                      return auditsForUser.map((audit, i) => (
                                         <div key={i} className="p-4 flex items-center justify-between hover:bg-surface-dim transition-colors text-xs">
                                            <div className="flex items-center gap-3">
                                               <div className="w-1.5 h-1.5 rounded-full bg-blue-500 shadow-[0_0_8px_rgba(59,130,246,0.5)]" />
                                               <span className="text-white font-medium">{audit.action} {audit.resource_kind ? `(${audit.resource_kind})` : ''}</span>
                                            </div>
                                            <span className="text-[#8c90a1] font-mono text-[10px]">{audit.created_at ? new Date(audit.created_at).toLocaleString() : (audit.timestamp || 'Just now')}</span>
                                         </div>
                                      ));
                                   })()}
                                </div>
                             </section>

                             <section className="bg-surface-card border border-border-subtle rounded-2xl overflow-hidden shadow-2xl">
                                <div className="p-6 border-b border-border-subtle bg-surface-dim flex justify-between items-center">
                                   <h3 className="font-display text-lg font-semibold text-white flex items-center gap-3">
                                      <CreditCard size={18} className="text-blue-400" /> Transaction Ledger
                                   </h3>
                                   <button className="text-[10px] font-bold text-[#8c90a1] hover:text-white uppercase tracking-widest bg-surface-card border border-border-subtle px-3 py-1.5 rounded-lg transition-all">Audit Entries</button>
                                </div>
                                <div className="overflow-x-auto">
                                   <table className="w-full text-left">
                                      <thead className="bg-[#111416]/50 font-display text-[10px] font-bold text-[#424655] uppercase tracking-widest">
                                         <tr>
                                            <th className="py-4 px-8">Batch ID</th>
                                            <th className="py-4 px-8">Timestamp</th>
                                            <th className="py-4 px-8">State</th>
                                            <th className="py-4 px-8 text-right">Value</th>
                                         </tr>
                                      </thead>
                                      <tbody className="font-sans text-sm text-white divide-y divide-border-subtle/50">
                                         {userOrders.map(order => (
                                            <tr key={order.id} className="hover:bg-surface-dim transition-colors group">
                                               <td className="py-4 px-8 font-mono text-xs text-blue-400 font-bold">{order.id}</td>
                                               <td className="py-4 px-8 text-[#8c90a1] font-mono text-xs">{order.date}</td>
                                               <td className="py-4 px-8">
                                                  <span className={cn("px-2 py-0.5 rounded text-[9px] font-bold uppercase tracking-widest", 
                                                    order.status === 'Completed' ? "bg-blue-400/10 text-blue-400 border border-blue-400/20 shadow-[0_0_10px_rgba(59,130,246,0.1)]" : "bg-surface-dim text-[#424655] border border-border-subtle"
                                                  )}>{order.status}</span>
                                               </td>
                                               <td className="py-4 px-8 text-right font-mono font-bold text-white group-hover:text-blue-400 transition-colors">${order.amount.toFixed(2)}</td>
                                            </tr>
                                         ))}
                                         {!userOrders.length && (
                                           <tr>
                                              <td colSpan={100} className="py-12 text-center text-[#424655] italic">No transaction records present for this user. No payment module configured.</td>
                                           </tr>
                                         )}
                                      </tbody>
                                   </table>
                                </div>
                             </section>
                          </div>
                        </div>
                      );
                    })()}
                  </div>
                )}
              </motion.div>
            )}

            {activePage === 'feature-flags' && (
              <motion.div key="flags" initial={{ opacity: 0, scale: 0.98 }} animate={{ opacity: 1, scale: 1 }}>
                {!selectedFlagId ? (
                  <>
                    <PageHeader 
                      title="Feature Toggles" 
                      description="Control system variables and feature rollouts dynamically."
                      actions={
                        <button 
                          onClick={() => { setEditingFlag(null); setFormData({}); setIsFlagModalOpen(true); }}
                          className="px-4 py-2 bg-blue-500 text-black font-bold rounded-lg text-sm flex items-center gap-2 whitespace-nowrap"
                        >
                          <Plus size={18} /> Add Toggle
                        </button>
                      }
                    />
                    
                    {flags.length === 0 && (
                      <div className="bg-surface-card border border-dashed border-border-subtle rounded-xl p-10 text-center">
                        <Flag className="mx-auto text-[#424655] mb-3" size={32} />
                        <h3 className="font-display text-base font-semibold text-white">No flags yet</h3>
                        <p className="text-xs text-[#8c90a1] mt-1">
                          Feature flags live in the dedicated <span className="font-mono text-blue-400">bffx_feature_flag</span> store. Create one to start gating rollouts.
                        </p>
                        <p className="text-xs text-[#8c90a1] mt-2">
                          Migrated from a legacy <span className="font-mono text-blue-400">feature_flag</span> resource? Rows are auto-imported on first boot; refresh this page if you just upgraded the framework.
                        </p>
                        <button
                          onClick={() => { setEditingFlag(null); setFormData({}); setIsFlagModalOpen(true); }}
                          className="mt-4 inline-flex items-center gap-2 px-4 py-2 bg-blue-500 text-black font-bold rounded-lg text-xs"
                        ><Plus size={14} /> Create your first flag</button>
                      </div>
                    )}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                      {flags.map(flag => (
                        <div key={flag.id} onClick={() => setSelectedFlagId(flag.id)} className="bg-surface-card border border-border-subtle rounded-xl p-6 group hover:border-blue-400/50 transition-all cursor-pointer">
                          <div className="flex justify-between items-start mb-4">
                            <div className="flex-1 mr-4">
                              <p className="font-mono text-sm font-bold text-blue-400">{flag.key}</p>
                              <p className="text-xs text-[#9BA3AF] mt-1 line-clamp-2">{flag.description}</p>
                            </div>
                            <button 
                               onClick={(e) => { e.stopPropagation(); handleToggleFlag(flag); }}
                               className={cn("w-12 h-6 rounded-full relative transition-colors flex-shrink-0", flag.enabled ? "bg-blue-500" : "bg-[#2D343C]")}
                            >
                              <motion.div animate={{ x: flag.enabled ? 24 : 4 }} className="absolute top-1 w-4 h-4 rounded-full bg-white shadow-lg" transition={{ type: "spring", stiffness: 500, damping: 30 }} />
                            </button>
                          </div>
                          <div className="flex justify-between items-center mt-6 pt-4 border-t border-border-subtle">
                             <span className="text-[10px] font-display font-bold uppercase text-[#8c90a1] tracking-widest">{flag.environment}</span>
                             <div className="flex gap-2">
                                <button onClick={(e) => { e.stopPropagation(); handleEditFlag(flag) }} className="text-[#8c90a1] hover:text-white transition-colors"><Edit2 size={14} /></button>
                                <button className="text-[#8c90a1] hover:text-red-400 transition-colors"><Trash2 size={14} /></button>
                                <ChevronRight size={14} className="text-[#424655] group-hover:text-blue-400" />
                             </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </>
                ) : activeFlag && (
                  <div className="space-y-6">
                    <div className="flex items-center gap-4 mb-8">
                       <button onClick={() => setSelectedFlagId(null)} className="p-2 bg-surface-card border border-border-subtle rounded-lg text-[#8c90a1] hover:text-white transition-all"><X size={20} /></button>
                       <div>
                          <h2 className="font-display text-4xl font-bold text-white tracking-tight">{activeFlag.key}</h2>
                          <p className="font-sans text-sm text-[#9BA3AF] mt-1">{activeFlag.description}</p>
                       </div>
                    </div>
 
                    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                       <div className="lg:col-span-2 space-y-6">
                          <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                             <h3 className="font-display text-lg font-semibold text-white mb-6">Targeting Rules</h3>
                             
                             <div className="space-y-8">
                                 <div className="space-y-4">
                                    <p className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-widest">Target Specific Users</p>
                                    <div className="bg-background border border-border-subtle rounded-lg p-4">
                                       <div className="flex flex-wrap gap-2 mb-3">
                                          {targetingEdit?.users.map(u => (
                                            <span key={u} className="px-2 py-1 bg-blue-400/10 text-blue-400 rounded-md font-mono text-xs border border-blue-400/20 flex items-center gap-2">
                                              {u} <X size={10} className="cursor-pointer" onClick={() => setTargetingEdit({ ...targetingEdit, users: targetingEdit.users.filter(x => x !== u) })} />
                                            </span>
                                          ))}
                                       </div>
                                       <div className="relative">
                                          <Plus size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[#8c90a1]" />
                                          <input 
                                            type="text" 
                                            placeholder="Add user key (press Enter to add...)" 
                                            value={newUserKey}
                                            onChange={(e) => setNewUserKey(e.target.value)}
                                            onKeyDown={handleAddUserKey}
                                            className="w-full bg-surface-dim border border-border-subtle rounded-md pl-10 pr-4 py-2 text-sm text-white focus:border-blue-400 outline-none" 
                                          />
                                       </div>
                                    </div>
                                 </div>
 
                                 <div className="space-y-4">
                                    <p className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-widest">Percentage Rollout</p>
                                    <div className="bg-background border border-border-subtle rounded-xl p-8">
                                       <div className="flex justify-between items-end mb-4">
                                          <span className="text-4xl font-display font-bold text-blue-400">{targetingEdit?.rollout || 0}%</span>
                                          <span className="text-xs font-sans text-[#8c90a1]">Traffic Allocation</span>
                                       </div>
                                       <input 
                                         type="range" 
                                         min="0" 
                                         max="100" 
                                         value={targetingEdit?.rollout || 0} 
                                         onChange={(e) => setTargetingEdit(prev => prev ? { ...prev, rollout: parseInt(e.target.value) || 0 } : null)}
                                         className="w-full h-2 bg-surface-dim rounded-lg appearance-none cursor-pointer accent-blue-500 border border-border-subtle" 
                                       />
                                       <div className="flex justify-between mt-2 text-[10px] font-mono text-[#424655]">
                                          <span>0%</span>
                                          <span>50%</span>
                                          <span>100%</span>
                                       </div>
                                    </div>
                                 </div>
                             </div>
                          </section>
 
                          <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                             <div className="flex justify-between items-center mb-6">
                                 <h3 className="font-display text-lg font-semibold text-white">Segment Targeting</h3>
                                 <button className="text-xs font-bold text-blue-400 hover:underline">Manage Segments</button>
                             </div>
                             <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {['Beta Testers', 'Internal Employees', 'Pro Users', 'Legacy Accounts'].map(segment => {
                                  const isSelected = targetingEdit?.segments.includes(segment) || false;
                                  return (
                                    <div 
                                      key={segment} 
                                      onClick={() => handleToggleSegment(segment)}
                                      className={cn("flex items-center gap-3 p-4 bg-surface-dim border rounded-xl hover:border-blue-400 transition-all cursor-pointer", isSelected ? "border-blue-500" : "border-border-subtle")}
                                    >
                                       <div className={cn("w-5 h-5 rounded border flex items-center justify-center transition-colors", isSelected ? "bg-blue-500 border-blue-500 text-black" : "border-border-subtle text-transparent")}>
                                          <Plus size={12} />
                                       </div>
                                       <span className="font-sans text-sm text-on-surface-variant font-medium">{segment}</span>
                                    </div>
                                  );
                                })}
                             </div>
                          </section>
                       </div>
 
                       <div className="space-y-6">
                          <section className="bg-surface-card border border-border-subtle rounded-xl p-6">
                             <h4 className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-widest mb-6">Environment Status</h4>
                             <div className="space-y-4">
                                {['Production', 'Staging', 'Development'].map(env => (
                                  <div key={env} className="flex justify-between items-center">
                                     <span className="font-sans text-xs text-white">{env}</span>
                                     <div className={cn("w-10 h-5 rounded-full relative", env === activeFlag.environment ? "bg-blue-500" : "bg-[#2D343C]")}>
                                        <div className={cn("absolute top-0.5 w-4 h-4 rounded-full bg-white", env === activeFlag.environment ? "right-0.5" : "left-0.5")} />
                                     </div>
                                  </div>
                                ))}
                             </div>
                          </section>
 
                          <div className="bg-blue-500/10 border border-blue-400/20 rounded-xl p-6">
                             <div className="flex items-center gap-3 text-blue-400 mb-2">
                                <Activity size={18} />
                                <span className="font-display font-bold text-sm">Real-time Usage</span>
                             </div>
                             <p className="text-xs text-[#8c90a1]">This flag was evaluated <span className="text-white font-bold">14,293</span> times in the last 15 minutes.</p>
                          </div>
 
                          <button 
                            onClick={saveTargetingChanges}
                            className="w-full py-4 bg-blue-500 text-black font-bold rounded-xl text-sm shadow-xl shadow-blue-500/10 flex items-center justify-center gap-2 hover:bg-blue-400 active:scale-[0.98] transition-all"
                          >
                             <Save size={18} /> Save Targeting Changes
                          </button>
                       </div>
                    </div>
                  </div>
                )}
              </motion.div>
            )}

            {activePage === 'performance' && (
              <motion.div key="performance" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
                <PageHeader title="Performance Vitals" description="Telemetrics and server health across all clusters." />
                
                <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
                  <div className="lg:col-span-2 space-y-6">
                    <div className="bg-surface-card border border-border-subtle rounded-xl p-6">
                        <h3 className="font-display text-lg font-semibold text-white mb-6">Service Latency (ms)</h3>
                        <div className="h-[250px]">
                          <ResponsiveContainer width="100%" height="100%">
                            <AreaChart data={perfData}>
                              <CartesianGrid strokeDasharray="3 3" stroke="#2D343C" vertical={false} />
                              <XAxis dataKey="time" stroke="#8c90a1" fontSize={10} axisLine={false} tickLine={false} />
                              <YAxis stroke="#8c90a1" fontSize={10} axisLine={false} tickLine={false} />
                              <Tooltip contentStyle={{ backgroundColor: '#1A1F24', border: '1px solid #2D343C' }} />
                              <Area type="stepBefore" dataKey="latency" stroke="#9cf132" strokeWidth={2} fill="#9cf132" fillOpacity={0.1} />
                            </AreaChart>
                          </ResponsiveContainer>
                        </div>
                    </div>
                    <div className="bg-surface-card border border-border-subtle rounded-xl p-6">
                        <h3 className="font-display text-lg font-semibold text-white mb-6">Cluster Load Distribution</h3>
                        <div className="h-[250px]">
                        <ResponsiveContainer width="100%" height="100%">
                          <BarChart data={[
                            { name: 'Cluster-A', load: 45 },
                            { name: 'Cluster-B', load: 62 },
                            { name: 'Cluster-C', load: 28 },
                            { name: 'Cluster-D', load: 84},
                          ]}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#2D343C" vertical={false} />
                            <XAxis dataKey="name" stroke="#8c90a1" fontSize={10} axisLine={false} tickLine={false} />
                            <YAxis stroke="#8c90a1" fontSize={10} axisLine={false} tickLine={false} />
                            <Tooltip cursor={{fill: '#1d2022'}} contentStyle={{ backgroundColor: '#1A1F24', border: '1px solid #2D343C' }} />
                            <Bar dataKey="load" radius={[4, 4, 0, 0]}>
                              {[45, 62, 28, 84].map((val, index) => (
                                <Cell key={`cell-${index}`} fill={val > 70 ? '#ffb4ab' : '#3b82f6'} />
                              ))}
                            </Bar>
                          </BarChart>
                        </ResponsiveContainer>
                        </div>
                    </div>
                  </div>
                  <div className="space-y-6">
                    <div className="bg-surface-card border border-border-subtle rounded-xl p-6">
                      <h3 className="font-display text-lg font-semibold text-white mb-6">Real-time Telemetry</h3>
                      <div className="space-y-4">
                        {[
                          { label: 'CPU Usage', value: '42%' },
                          { label: 'Memory', value: '8.4/16 GB' },
                          { label: 'Disk I/O', value: '124 MB/s' },
                          { label: 'Open Files', value: '1,492' },
                        ].map(item => (
                          <div key={item.label} className="p-4 bg-surface-dim border border-border-subtle rounded-lg">
                            <p className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-widest">{item.label}</p>
                            <p className="text-xl font-display font-bold text-white mt-1">{item.value}</p>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                </div>
              </motion.div>
            )}

            {activePage === 'alerts' && (
              <motion.div key="alerts" initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
                <PageHeader title="Incident Alerts" description="Live feed of system warnings and critical failures." />
                
                <div className="space-y-4">
                  {incidents.length === 0 ? (
                    <div className="text-center py-12 bg-surface-card rounded-xl border border-border-subtle p-8">
                      <ShieldAlert className="mx-auto text-blue-400/40 mb-4" size={48} />
                      <p className="text-white font-medium text-lg">No active incidents</p>
                      <p className="text-[#8c90a1] text-sm mt-1">All systems are operational.</p>
                    </div>
                  ) : (
                    incidents.map(inc => {
                      const isCritical = (inc.severity || '').toLowerCase() === 'critical';
                      const isWarning = (inc.severity || '').toLowerCase() === 'warning';
                      const cardBorder = isCritical ? "border-red-400" : isWarning ? "border-orange-400" : "border-blue-400";
                      const iconBg = isCritical ? "bg-red-400/10 text-red-400" : isWarning ? "bg-orange-400/10 text-orange-400" : "bg-blue-400/10 text-blue-400";

                      return (
                        <div key={inc.id} className={cn(
                          "group border-l-4 p-6 bg-surface-card rounded-r-xl transition-all hover:bg-surface-dim",
                          cardBorder
                        )}>
                          <div className="flex justify-between items-start">
                            <div className="flex gap-4 flex-1">
                              <div className={cn("w-10 h-10 rounded-lg flex items-center justify-center shrink-0", iconBg)}>
                                {isCritical ? <ShieldAlert size={20} /> : isWarning ? <AlertTriangle size={20} /> : <Bell size={20} />}
                              </div>
                              <div className="flex-1 min-w-0">
                                <p className="font-display font-semibold text-white break-words">{inc.title || 'Unknown Incident'}</p>
                                <div className="flex gap-3 mt-1 items-center flex-wrap">
                                  <span className="font-mono text-[10px] text-[#8c90a1] uppercase tracking-wider">Severity: {inc.severity || 'Info'}</span>
                                  <span className="w-1 h-1 rounded-full bg-[#2D343C]" />
                                  <span className="font-mono text-[10px] text-[#8c90a1]">ID: {inc.id}</span>
                                </div>
                                {inc.stack_trace && (
                                  <pre className="mt-3 p-3 bg-[#0d0e10] border border-border-subtle rounded-lg text-xs font-mono text-red-300 max-h-40 overflow-y-auto whitespace-pre-wrap break-all">
                                    {inc.stack_trace}
                                  </pre>
                                )}
                              </div>
                            </div>
                            <div className="opacity-0 group-hover:opacity-100 transition-opacity flex gap-2 ml-4 shrink-0">
                              <button 
                                onClick={() => handleResolveIncident(inc.id)}
                                className="px-3 py-1.5 bg-[#111416] border border-border-subtle rounded-lg text-[10px] font-display font-bold text-white hover:border-blue-400 transition-all"
                              >
                                RESOLVE
                              </button>
                            </div>
                          </div>
                        </div>
                      );
                    })
                  )}
                </div>
              </motion.div>
            )}

            {activePage === 'resources' && (() => {
              const scopes = activeCollection.spec?.index?.scopes || [{ name: 'all', default: true }];
              const filters = activeCollection.spec?.index?.filters || [];
              const stringFilter = activeCollection.spec?.index?.filters?.find((f: any) => f.as === 'string');
              
              const totalPages = Math.ceil(resourceTotalCount / resourceQueryParams.perPage) || 1;
              const pageNumbers = [];
              const startPage = Math.max(1, resourceQueryParams.page - 2);
              const endPage = Math.min(totalPages, resourceQueryParams.page + 2);
              for (let i = startPage; i <= endPage; i++) {
                pageNumbers.push(i);
              }

              const handleSearch = (val: string) => {
                if (!stringFilter) return;
                setResourceQueryParams(prev => ({
                  ...prev,
                  page: 1,
                  filters: {
                    ...prev.filters,
                    [`filter_${stringFilter.field}_contains`]: val
                  }
                }));
              };

              const currentSearchVal = stringFilter ? (resourceQueryParams.filters[`filter_${stringFilter.field}_contains`] || '') : '';

              return (
                <motion.div key="resources" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
                  <PageHeader 
                    title="Resource Explorer" 
                    description="Inspect database collections and associated objects." 
                    actions={
                      <div className="flex gap-3 flex-wrap">
                        {(activeCollection.spec?.collection_actions || []).map((act: any) => (
                          <button
                            key={act.name}
                            onClick={() => handleCollectionAction(act)}
                            className="px-4 py-2 bg-[#111416] border border-border-subtle hover:border-blue-400 text-white rounded-lg text-sm font-semibold transition-all whitespace-nowrap"
                          >
                            {act.label || act.name}
                          </button>
                        ))}
                        <div className="flex items-center gap-3 px-4 py-2 bg-surface-card border border-border-subtle rounded-lg">
                          <span className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-widest">Pin to Menu</span>
                          <button 
                            onClick={() => togglePinning(activeCollection.id)}
                            className={cn("w-10 h-5 rounded-full relative transition-colors", activeCollection.pinnedToMenu ? "bg-blue-500" : "bg-[#2D343C]")}
                          >
                            <motion.div animate={{ x: activeCollection.pinnedToMenu ? 20 : 2 }} className="absolute top-0.5 w-4 h-4 rounded-full bg-white shadow-lg" transition={{ type: "spring", stiffness: 500, damping: 30 }} />
                          </button>
                        </div>
                        <button 
                          onClick={() => { setEditingResource(null); setFormData({}); setIsResourceModalOpen(true); }}
                          className="px-4 py-2 bg-blue-500 text-black font-bold rounded-lg text-sm flex items-center gap-2 whitespace-nowrap"
                        >
                          <Plus size={18} /> New Entry
                        </button>
                      </div>
                    }
                  />
                  
                  {/* Scopes Tab Bar */}
                  {scopes.length > 0 && (
                    <div className="flex border-b border-border-subtle mb-6 overflow-x-auto select-none gap-2">
                      {scopes.map((sc: any) => (
                        <button
                          key={sc.name}
                          onClick={() => setResourceQueryParams(prev => ({ ...prev, page: 1, scope: sc.name }))}
                          className={cn(
                            "px-4 py-2.5 font-display text-xs font-bold uppercase tracking-widest border-b-2 transition-all whitespace-nowrap",
                            resourceQueryParams.scope === sc.name
                              ? "border-blue-500 text-blue-400 bg-blue-500/5"
                              : "border-transparent text-[#8c90a1] hover:text-white"
                          )}
                        >
                          {sc.name}
                        </button>
                      ))}
                    </div>
                  )}

                  <div className="grid grid-cols-1 xl:grid-cols-5 gap-8">
                    {/* Collections Sidebar */}
                    <div className="xl:col-span-1 space-y-2">
                      <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest px-2 mb-4">Collections</p>
                      {collections.map(coll => (
                        <button 
                          key={coll.id} 
                          onClick={() => setSelectedCollectionId(coll.id)}
                          className={cn(
                            "w-full text-left px-4 py-2.5 rounded-lg font-mono text-xs transition-all",
                            coll.id === selectedCollectionId ? "bg-blue-400/10 text-blue-400 border border-blue-400/20 shadow-lg shadow-blue-500/5" : "text-[#8c90a1] hover:bg-surface-card hover:text-white"
                          )}
                        >
                          {humanizeResourceName(coll.name, coll.displayName)}
                        </button>
                      ))}
                      <button className="w-full text-left px-4 py-2.5 rounded-lg font-mono text-[10px] text-[#424655] italic hover:text-blue-400 transition-all">+ Create Collection</button>
                    </div>

                    {/* Table Area and Filters Sidebar */}
                    <div className={cn("xl:col-span-4 grid gap-6", isFilterSidebarOpen && filters.length > 0 ? "grid-cols-1 lg:grid-cols-4" : "grid-cols-1")}>
                      <div className={cn(isFilterSidebarOpen && filters.length > 0 ? "lg:col-span-3" : "")}>
                        <div className="bg-surface-card border border-border-subtle rounded-xl overflow-hidden shadow-2xl">
                           <div className="p-4 bg-surface-dim border-b border-border-subtle flex justify-between items-center flex-wrap gap-4">
                              {selectedRowIds.length > 0 ? (
                                <div className="flex items-center gap-4 text-xs">
                                  <span className="font-bold text-blue-400">{selectedRowIds.length} records selected</span>
                                  <div className="flex gap-2">
                                    {(activeCollection.spec?.batch_actions || []).map((act: any) => (
                                      <button
                                        key={act.name}
                                        onClick={() => handleBatchAction(act)}
                                        className="px-3 py-1.5 bg-blue-500 text-black font-bold rounded hover:bg-blue-400 transition-all text-[10px] uppercase tracking-wider"
                                      >
                                        {act.label || act.name}
                                      </button>
                                    ))}
                                    <button
                                      onClick={() => setSelectedRowIds([])}
                                      className="px-3 py-1.5 bg-[#111416] border border-border-subtle text-white rounded hover:border-red-400 hover:text-red-400 transition-all text-[10px] uppercase tracking-wider"
                                    >
                                      Cancel
                                    </button>
                                  </div>
                                </div>
                              ) : (
                                <div className="flex gap-3">
                                  {stringFilter && (
                                    <div className="relative">
                                       <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[#8c90a1]" />
                                       <input 
                                         type="text" 
                                         value={currentSearchVal}
                                         onChange={(e) => handleSearch(e.target.value)}
                                         placeholder={`Search ${humanizeResourceName(activeCollection.name, activeCollection.displayName)}...`} 
                                         className="bg-background border border-border-subtle rounded-lg pl-9 pr-4 py-1.5 text-xs text-white w-48 focus:border-blue-400 outline-none" 
                                       />
                                    </div>
                                  )}
                                  {filters.length > 0 && (
                                    <button 
                                      onClick={() => setIsFilterSidebarOpen(prev => !prev)}
                                      className={cn(
                                        "px-3 py-1.5 border rounded-lg text-[10px] font-display font-bold flex items-center gap-2 hover:border-blue-400 transition-all",
                                        isFilterSidebarOpen ? "bg-blue-500/10 border-blue-500 text-blue-400" : "bg-[#111416] border-border-subtle text-white"
                                      )}
                                    >
                                      <Filter size={12} /> Filters
                                    </button>
                                  )}
                                </div>
                              )}
                              <div className="flex items-center gap-2 text-[10px] font-mono text-[#8c90a1]">
                                 <span>Count: {resourceTotalCount} items</span>
                              </div>
                           </div>
                           
                           <div className="overflow-x-auto">
                            <table className="w-full text-left">
                               <thead className="bg-[#111416] border-b border-border-subtle font-display text-[10px] font-bold text-[#9BA3AF] uppercase tracking-widest">
                                 <tr>
                                   {activeCollection.spec?.index?.selectable && (
                                     <th className="py-4 px-6 w-12 text-center select-none">
                                       <input
                                         type="checkbox"
                                         checked={activeCollection.data.length > 0 && selectedRowIds.length === activeCollection.data.length}
                                         onChange={(e) => {
                                           if (e.target.checked) {
                                             setSelectedRowIds(activeCollection.data.map(r => String(r.id)));
                                           } else {
                                             setSelectedRowIds([]);
                                           }
                                         }}
                                         className="rounded border-border-subtle text-blue-500 bg-background focus:ring-0 focus:ring-offset-0"
                                       />
                                     </th>
                                   )}
                                   <th className="py-4 px-6 cursor-pointer hover:text-white transition-all select-none" onClick={() => handleSort('id')}>
                                     <div className="flex items-center gap-1.5">
                                       ID
                                       {resourceQueryParams.sort === 'id_asc' && <ChevronUp size={12} className="text-blue-400" />}
                                       {resourceQueryParams.sort === 'id_desc' && <ChevronDown size={12} className="text-blue-400" />}
                                     </div>
                                   </th>
                                   {visibleFields.map(f => (
                                     <th 
                                       key={f.name} 
                                       className="py-4 px-6 cursor-pointer hover:text-white transition-all select-none"
                                       onClick={() => handleSort(f.name)}
                                     >
                                       <div className="flex items-center gap-1.5">
                                         {humanizeResourceName(f.name)}
                                         {resourceQueryParams.sort === `${f.name}_asc` && <ChevronUp size={12} className="text-blue-400" />}
                                         {resourceQueryParams.sort === `${f.name}_desc` && <ChevronDown size={12} className="text-blue-400" />}
                                       </div>
                                     </th>
                                   ))}
                                   <th className="py-4 px-6 text-right">Actions</th>
                                 </tr>
                               </thead>
                               <tbody className="font-sans text-sm text-on-surface-variant divide-y divide-border-subtle">
                                 {activeCollection.data.map((row, i) => (
                                   <tr key={row.id} className="hover:bg-surface-dim transition-colors group">
                                     {activeCollection.spec?.index?.selectable && (
                                       <td className="py-4 px-6 text-center w-12">
                                         <input
                                           type="checkbox"
                                           checked={selectedRowIds.includes(String(row.id))}
                                           onChange={(e) => {
                                             if (e.target.checked) {
                                               setSelectedRowIds([...selectedRowIds, String(row.id)]);
                                             } else {
                                               setSelectedRowIds(selectedRowIds.filter(id => id !== String(row.id)));
                                             }
                                           }}
                                           className="rounded border-border-subtle text-blue-500 bg-background focus:ring-0 focus:ring-offset-0"
                                         />
                                       </td>
                                     )}
                                     <td className="py-4 px-6 text-blue-400 font-mono text-xs font-bold">#{row.id}</td>
                                     {visibleFields.map(f => {
                                       const belongs = (activeCollection.spec?.belongs_to || []).find(
                                         (b: { field: string }) => b.field === f.name,
                                       );
                                       const fieldDef = (activeCollection.fields || []).find(
                                         (fd: { name: string }) => fd.name === f.name,
                                       );
                                       const relTarget = belongs?.resource || fieldDef?.target;
                                       const cellVal = row[f.name];
                                       return (
                                       <td key={f.name} className="py-4 px-6 max-w-xs truncate">
                                          {relTarget && cellVal != null && cellVal !== '' ? (
                                            <RelationLink
                                              targetResource={relTarget}
                                              recordId={String(cellVal)}
                                              onOpen={openRelatedResource}
                                            />
                                          ) : typeof cellVal === 'object' && cellVal !== null ? (
                                            <span className="font-mono text-xs bg-[#0d0e10] px-1.5 py-0.5 rounded text-[#8c90a1] border border-border-subtle">
                                              {JSON.stringify(cellVal)}
                                            </span>
                                          ) : typeof cellVal === 'number' ? (
                                            <span className="font-mono text-white">{cellVal}</span>
                                          ) : typeof cellVal === 'boolean' ? (
                                            <span className={cn("px-1.5 py-0.5 text-[10px] font-bold uppercase rounded", cellVal ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400")}>
                                              {cellVal ? 'true' : 'false'}
                                            </span>
                                          ) : (
                                            String(cellVal ?? '')
                                          )}
                                       </td>
                                     );})}
                                     <td className="py-4 px-6 text-right">
                                        <div className="flex justify-end gap-2 text-[#8c90a1] opacity-0 group-hover:opacity-100 transition-opacity flex-wrap">
                                           {(activeCollection.spec?.member_actions || []).map((act: any) => (
                                             <button
                                               key={act.name}
                                               onClick={() => handleMemberAction(act, row.id)}
                                               className="px-2 py-1 text-[9px] font-bold uppercase tracking-wider bg-surface-card hover:bg-blue-500 hover:text-black border border-border-subtle hover:border-blue-500 rounded text-[#8c90a1] transition-all"
                                               title={act.label || act.name}
                                             >
                                               {act.label || act.name}
                                             </button>
                                           ))}
                                           <button onClick={() => { setShowingResource(row); setIsShowModalOpen(true); }} className="p-1.5 hover:text-green-400 hover:bg-green-400/10 rounded transition-all"><Eye size={12} /></button>
                                           <button onClick={() => { setEditingResource(row); setFormData(row); setIsResourceModalOpen(true); }} className="p-1.5 hover:text-blue-400 hover:bg-blue-400/10 rounded transition-all"><Edit2 size={12} /></button>
                                           <button onClick={() => handleDeleteResource(row)} className="p-1.5 hover:text-red-400 hover:bg-red-400/10 rounded transition-all"><Trash2 size={12} /></button>
                                        </div>
                                     </td>
                                   </tr>
                                 ))}
                                 {activeCollection.data.length === 0 && (
                                   <tr>
                                      <td colSpan={100} className="py-12 text-center text-[#424655] italic">No records found in this collection.</td>
                                   </tr>
                                 )}
                               </tbody>
                            </table>
                           </div>

                           {/* Pagination Footer */}
                           <div className="p-4 bg-surface-dim border-t border-border-subtle flex flex-col sm:flex-row justify-between items-center gap-4 text-xs select-none">
                             <div className="flex items-center gap-4 text-[#8c90a1]">
                               <span>
                                 Showing {resourceTotalCount === 0 ? 0 : Math.min(resourceTotalCount, (resourceQueryParams.page - 1) * resourceQueryParams.perPage + 1)} to{' '}
                                 {Math.min(resourceTotalCount, resourceQueryParams.page * resourceQueryParams.perPage)} of {resourceTotalCount} entries
                               </span>
                               <div className="flex items-center gap-2">
                                 <span>Per page:</span>
                                 <select
                                   value={resourceQueryParams.perPage}
                                   onChange={(e) => {
                                     const val = parseInt(e.target.value, 10);
                                     setResourceQueryParams(prev => ({
                                       ...prev,
                                       page: 1,
                                       perPage: val
                                     }));
                                   }}
                                   className="bg-background border border-border-subtle rounded px-2 py-1 text-white focus:border-blue-400 outline-none"
                                 >
                                   {[10, 25, 50, 100].map(size => (
                                     <option key={size} value={size}>{size}</option>
                                   ))}
                                 </select>
                               </div>
                             </div>

                             {totalPages > 1 && (
                               <div className="flex items-center gap-1.5">
                                 <button
                                   disabled={resourceQueryParams.page <= 1}
                                   onClick={() => setResourceQueryParams(prev => ({ ...prev, page: prev.page - 1 }))}
                                   className="p-1.5 bg-[#111416] border border-border-subtle rounded hover:border-blue-400 text-white disabled:opacity-50 disabled:pointer-events-none transition-all"
                                 >
                                   <ChevronLeft size={16} />
                                 </button>
                                 {pageNumbers.map(pageNum => (
                                   <button
                                     key={pageNum}
                                     onClick={() => setResourceQueryParams(prev => ({ ...prev, page: pageNum }))}
                                     className={cn(
                                       "px-3 py-1.5 border rounded font-bold transition-all",
                                       resourceQueryParams.page === pageNum
                                         ? "bg-blue-500 border-blue-500 text-black"
                                         : "bg-[#111416] border-border-subtle text-white hover:border-blue-400"
                                     )}
                                   >
                                     {pageNum}
                                   </button>
                                 ))}
                                 <button
                                   disabled={resourceQueryParams.page >= totalPages}
                                   onClick={() => setResourceQueryParams(prev => ({ ...prev, page: prev.page + 1 }))}
                                   className="p-1.5 bg-[#111416] border border-border-subtle rounded hover:border-blue-400 text-white disabled:opacity-50 disabled:pointer-events-none transition-all"
                                 >
                                   <ChevronRight size={16} />
                                 </button>
                               </div>
                             )}
                           </div>
                        </div>
                      </div>

                      {/* Filters Sidebar Drawer */}
                      {isFilterSidebarOpen && filters.length > 0 && (
                        <div className="lg:col-span-1 bg-surface-card border border-border-subtle rounded-xl p-4 space-y-4 shadow-2xl h-fit">
                          <div className="flex justify-between items-center border-b border-border-subtle pb-2">
                            <h4 className="text-xs font-display font-bold text-white uppercase tracking-widest flex items-center gap-2">
                              <Filter size={12} /> Filters
                            </h4>
                            <button onClick={() => setIsFilterSidebarOpen(false)} className="text-[#8c90a1] hover:text-white transition-all">
                              <X size={14} />
                            </button>
                          </div>

                          <div className="space-y-4">
                            {filters.map((f: any) => {
                              const labelText = f.label || humanizeResourceName(f.field);
                              
                              if (f.as === 'string') {
                                const key = `filter_${f.field}_contains`;
                                return (
                                  <div key={f.field} className="space-y-1">
                                    <label className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-wider">{labelText}</label>
                                    <input
                                      type="text"
                                      value={localFilters[key] || ''}
                                      onChange={(e) => setLocalFilters(prev => ({ ...prev, [key]: e.target.value }))}
                                      placeholder="Contains..."
                                      className="w-full bg-background border border-border-subtle rounded-lg px-3 py-1.5 text-xs text-white focus:border-blue-400 outline-none"
                                    />
                                  </div>
                                );
                              }

                              if (f.as === 'select') {
                                const key = `filter_${f.field}`;
                                const opts = f.options || [];
                                return (
                                  <div key={f.field} className="space-y-1">
                                    <label className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-wider">{labelText}</label>
                                    <select
                                      value={localFilters[key] || ''}
                                      onChange={(e) => setLocalFilters(prev => ({ ...prev, [key]: e.target.value }))}
                                      className="w-full bg-background border border-border-subtle rounded-lg px-3 py-1.5 text-xs text-white focus:border-blue-400 outline-none"
                                    >
                                      <option value="">All</option>
                                      {opts.map((opt: string) => (
                                        <option key={opt} value={opt}>{opt}</option>
                                      ))}
                                    </select>
                                  </div>
                                );
                              }

                              if (f.as === 'bool') {
                                const key = `filter_${f.field}`;
                                return (
                                  <div key={f.field} className="space-y-1">
                                    <label className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-wider">{labelText}</label>
                                    <select
                                      value={localFilters[key] || ''}
                                      onChange={(e) => setLocalFilters(prev => ({ ...prev, [key]: e.target.value }))}
                                      className="w-full bg-background border border-border-subtle rounded-lg px-3 py-1.5 text-xs text-white focus:border-blue-400 outline-none"
                                    >
                                      <option value="">All</option>
                                      <option value="true">True</option>
                                      <option value="false">False</option>
                                    </select>
                                  </div>
                                );
                              }

                              if (f.as === 'date_range') {
                                const keyGte = `filter_${f.field}_gte`;
                                const keyLte = `filter_${f.field}_lte`;
                                return (
                                  <div key={f.field} className="space-y-2">
                                    <label className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-wider">{labelText}</label>
                                    <div className="space-y-1">
                                      <span className="text-[9px] text-[#424655] font-semibold uppercase">After</span>
                                      <input
                                        type="date"
                                        value={localFilters[keyGte] || ''}
                                        onChange={(e) => setLocalFilters(prev => ({ ...prev, [keyGte]: e.target.value }))}
                                        className="w-full bg-background border border-border-subtle rounded-lg px-3 py-1.5 text-xs text-white focus:border-blue-400 outline-none"
                                      />
                                    </div>
                                    <div className="space-y-1">
                                      <span className="text-[9px] text-[#424655] font-semibold uppercase">Before</span>
                                      <input
                                        type="date"
                                        value={localFilters[keyLte] || ''}
                                        onChange={(e) => setLocalFilters(prev => ({ ...prev, [keyLte]: e.target.value }))}
                                        className="w-full bg-background border border-border-subtle rounded-lg px-3 py-1.5 text-xs text-white focus:border-blue-400 outline-none"
                                      />
                                    </div>
                                  </div>
                                );
                              }

                              return null;
                            })}
                          </div>

                          <div className="flex gap-2 pt-2 border-t border-border-subtle">
                            <button
                              onClick={() => {
                                setResourceQueryParams(prev => ({
                                  ...prev,
                                  page: 1,
                                  filters: localFilters
                                }));
                              }}
                              className="flex-1 py-2 bg-blue-500 text-black text-xs font-bold rounded-lg text-center hover:bg-blue-400 transition-all"
                            >
                              Apply
                            </button>
                            <button
                              onClick={() => {
                                setLocalFilters({});
                                setResourceQueryParams(prev => ({
                                  ...prev,
                                  page: 1,
                                  filters: {}
                                }));
                              }}
                              className="px-3 py-2 bg-[#111416] border border-border-subtle text-white text-xs font-bold rounded-lg text-center hover:border-red-400 hover:text-red-400 transition-all"
                            >
                              Reset
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                </motion.div>
              );
            })()}

            {activePage === 'app-features' && (
              <motion.div key="app-features" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="flex flex-col h-full">
                <PageHeader
                  title="App Features"
                  description="Screens, Actions, and Sources powering the mobile (and future web) experience."
                  actions={
                    selectedFeatureNavPage ? (
                      <div className="flex items-center gap-3 px-4 py-2 bg-surface-card border border-border-subtle rounded-lg">
                        <span className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-widest">Pin to Menu</span>
                        <button
                          type="button"
                          onClick={() => toggleFeaturePinning()}
                          className={cn(
                            'w-10 h-5 rounded-full relative transition-colors',
                            isSelectedFeaturePinned ? 'bg-blue-500' : 'bg-[#2D343C]',
                          )}
                        >
                          <motion.div
                            animate={{ x: isSelectedFeaturePinned ? 20 : 2 }}
                            className="absolute top-0.5 w-4 h-4 rounded-full bg-white shadow-lg"
                            transition={{ type: 'spring', stiffness: 500, damping: 30 }}
                          />
                        </button>
                      </div>
                    ) : null
                  }
                />

                {featureTreeError && (
                  <div className="mt-4 rounded border border-red-500/40 bg-red-500/10 text-red-300 px-4 py-3 text-sm">
                    Failed to load features: {featureTreeError}
                  </div>
                )}

                {featureTree.orphan_actions.length > 0 && !dismissedOrphanBanner && featureTree.feature_clusters.length === 0 && (
                  <div className="mt-4 rounded border border-yellow-500/40 bg-yellow-500/10 text-yellow-200 px-4 py-3 text-sm flex items-start gap-3">
                    <div className="flex-1">
                      <p className="font-semibold">{featureTree.orphan_actions.length} orphan action{featureTree.orphan_actions.length === 1 ? '' : 's'} detected.</p>
                      <p className="text-xs mt-1 text-yellow-200/80">
                        These Actions are declared in manifests but not referenced by any Screen. They may be safe to delete.
                      </p>
                      <ul className="mt-2 text-xs space-y-0.5">
                        {featureTree.orphan_actions.map(o => (
                          <li key={o.name}><span className="font-mono">{o.name}</span> <span className="text-yellow-200/60">— {o.route_method} {o.route_path}</span></li>
                        ))}
                      </ul>
                    </div>
                    <button
                      className="text-yellow-200/80 hover:text-yellow-200 text-xs underline"
                      onClick={() => setDismissedOrphanBanner(true)}
                    >Dismiss</button>
                  </div>
                )}

                <div className="flex gap-6 flex-1 mt-4 min-h-0">
                  {/* Tree */}
                  <div className="w-72 shrink-0 overflow-y-auto pr-2 border-r border-border-subtle">
                    {featureTreeLoading && (
                      <p className="text-xs text-text-muted px-2 py-1">Loading…</p>
                    )}
                    {!featureTreeLoading && featureTree.groups.length === 0 && featureTree.feature_clusters.length === 0 && (
                      <p className="text-xs text-text-muted px-2 py-1">No screens or features declared.</p>
                    )}

                    {/* Screens */}
                    {featureTree.groups.map(group => (
                      <div key={group.name} className="mb-4">
                        <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest px-2">
                          Screens — {group.display_name} <span className="text-text-muted/60">({group.screens.length})</span>
                        </p>
                        <ul className="mt-1">
                          {group.screens.map(s => (
                            <li key={s.name}>
                              <button
                                onClick={() => { setSelectedScreenName(s.name); setSelectedClusterName(null); }}
                                className={`w-full text-left px-3 py-1.5 rounded text-sm flex items-center gap-2 ${
                                  selectedScreenName === s.name
                                    ? 'bg-surface-card text-text-main'
                                    : 'text-text-muted hover:bg-surface-card/60'
                                }`}
                              >
                                <Code2 size={14} className="opacity-60" />
                                <span className="flex-1 truncate font-mono">{s.name}</span>
                                {s.stream && <Radio size={12} className="text-green-400" />}
                              </button>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ))}

                    {/* Feature clusters derived from Actions */}
                    {featureTree.feature_clusters.length > 0 && (
                      <div className="mb-4">
                        <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest px-2">
                          Features <span className="text-text-muted/60">({featureTree.feature_clusters.length})</span>
                        </p>
                        <ul className="mt-1">
                          {featureTree.feature_clusters.map(c => {
                            const orphanCount = c.actions.filter(a => a.is_orphan).length;
                            return (
                              <li key={c.name}>
                                <button
                                  onClick={() => { setSelectedClusterName(c.name); setSelectedScreenName(null); }}
                                  className={`w-full text-left px-3 py-1.5 rounded text-sm flex items-center gap-2 ${
                                    selectedClusterName === c.name
                                      ? 'bg-surface-card text-text-main'
                                      : 'text-text-muted hover:bg-surface-card/60'
                                  }`}
                                  title={orphanCount > 0 ? `${orphanCount} action(s) not wired to any Screen` : undefined}
                                >
                                  <ComponentIcon size={14} className="opacity-60" />
                                  <span className="flex-1 truncate">{c.display_name}</span>
                                  <span className="text-[10px] text-text-muted/60 font-mono">{c.actions.length}</span>
                                  {orphanCount > 0 && (
                                    <span className="w-1.5 h-1.5 rounded-full bg-yellow-400" title={`${orphanCount} orphan action(s)`} />
                                  )}
                                </button>
                              </li>
                            );
                          })}
                        </ul>
                      </div>
                    )}
                  </div>

                  {/* Detail */}
                  <div className="flex-1 overflow-y-auto min-w-0">
                    {(() => {
                      const cluster = featureTree.feature_clusters.find(c => c.name === selectedClusterName);
                      if (cluster) {
                        const orphanCount = cluster.actions.filter(a => a.is_orphan).length;
                        return (
                          <div className="space-y-4">
                            <div>
                              <h3 className="font-display text-xl font-bold text-text-main">{cluster.display_name}</h3>
                              <p className="text-xs text-text-muted font-mono mt-1">
                                feature group · {cluster.group || 'mobile'} · {cluster.actions.length} action{cluster.actions.length === 1 ? '' : 's'}
                                {orphanCount > 0 && <span className="text-yellow-400 ml-2">{orphanCount} not wired to a Screen</span>}
                              </p>
                            </div>

                            {cluster.screens_using && cluster.screens_using.length > 0 && (
                              <div>
                                <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-2">Used By Screens</p>
                                <div className="flex flex-wrap gap-1.5">
                                  {cluster.screens_using.map(s => (
                                    <button
                                      key={s}
                                      onClick={() => { setSelectedScreenName(s); setSelectedClusterName(null); }}
                                      className="px-2 py-1 rounded bg-blue-500/10 text-blue-300 text-xs font-mono hover:bg-blue-500/20"
                                    >{s}</button>
                                  ))}
                                </div>
                              </div>
                            )}

                            <div>
                              <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-2">Actions ({cluster.actions.length})</p>
                              <ul className="space-y-1">
                                {cluster.actions.map(a => (
                                  <li key={a.name} className="text-xs flex items-center gap-2 px-2 py-1.5 rounded bg-surface-card/50">
                                    <span className={`px-1.5 py-0.5 rounded text-[10px] uppercase font-bold tracking-wider ${
                                      a.is_orphan ? 'bg-yellow-500/20 text-yellow-300' : 'bg-blue-500/20 text-blue-300'
                                    }`}>{a.is_orphan ? 'orphan' : 'wired'}</span>
                                    <span className="font-mono">{a.name}</span>
                                    {a.route_path && (
                                      <span className="text-text-muted/60">— {a.route_method} {a.route_path}</span>
                                    )}
                                  </li>
                                ))}
                              </ul>
                            </div>

                            {cluster.touches_resources && cluster.touches_resources.length > 0 && (
                              <div>
                                <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-2">Touches Resources</p>
                                <div className="flex flex-wrap gap-1.5">
                                  {cluster.touches_resources.map(r => (
                                    <span key={r} className="px-2 py-1 rounded bg-purple-500/10 text-purple-300 text-xs font-mono">{r}</span>
                                  ))}
                                </div>
                              </div>
                            )}
                          </div>
                        );
                      }

                      const screen = featureTree.groups
                        .flatMap(g => g.screens)
                        .find(s => s.name === selectedScreenName);
                      if (!screen) {
                        return (
                          <p className="text-sm text-text-muted">Select a screen or feature on the left to view its catalog details.</p>
                        );
                      }
                      return (
                        <div className="space-y-4">
                          <div>
                            <h3 className="font-display text-xl font-bold text-text-main">{humanizeResourceName(screen.name)}</h3>
                            <p className="text-xs text-text-muted font-mono mt-1">
                              {screen.route_method} {screen.route_path}
                            </p>
                          </div>

                          <div className="grid grid-cols-2 gap-3">
                            <InfoCell label="Nav Type" value={screen.nav_type || '—'} />
                            <InfoCell label="Requires Auth" value={screen.requires_auth || '—'} />
                            <InfoCell label="Order" value={String(screen.order)} />
                            <InfoCell label="Streaming" value={screen.stream ? 'yes (eventbus)' : 'no'} />
                          </div>

                          <div>
                            <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-2">Sources ({screen.sources.length})</p>
                            <ul className="space-y-1">
                              {screen.sources.length === 0 && (
                                <li className="text-xs text-text-muted">No sources declared.</li>
                              )}
                              {screen.sources.map((src, idx) => (
                                <li key={idx} className="text-xs flex items-center gap-2 px-2 py-1 rounded bg-surface-card/50">
                                  <span className={`px-1.5 py-0.5 rounded text-[10px] uppercase font-bold tracking-wider ${
                                    src.kind === 'action' ? 'bg-blue-500/20 text-blue-300' :
                                    src.kind === 'resource' ? 'bg-purple-500/20 text-purple-300' :
                                    src.kind === 'builtin' ? 'bg-gray-500/20 text-gray-300' :
                                    'bg-red-500/20 text-red-300'
                                  }`}>{src.kind}</span>
                                  <span className="font-mono">{src.name}</span>
                                  {src.route_path && (
                                    <span className="text-text-muted/60 font-mono ml-auto">{src.route_method} {src.route_path}</span>
                                  )}
                                </li>
                              ))}
                            </ul>
                          </div>

                          <div>
                            <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-2">Sections ({screen.sections.length})</p>
                            <ul className="space-y-1">
                              {screen.sections.length === 0 && (
                                <li className="text-xs text-text-muted">No sections declared.</li>
                              )}
                              {screen.sections.map((sec, idx) => (
                                <li key={idx} className="text-xs flex items-center gap-2 px-2 py-1 rounded bg-surface-card/50">
                                  <span className="font-mono">{sec.key}</span>
                                  {sec.ui_type && <span className="text-text-muted/60">{sec.ui_type}</span>}
                                  <span className={`ml-auto text-[10px] uppercase tracking-wider ${sec.default_visible ? 'text-green-400' : 'text-text-muted/60'}`}>
                                    {sec.default_visible ? 'visible' : 'hidden'}
                                  </span>
                                </li>
                              ))}
                            </ul>
                          </div>

                          {screen.touches_resources.length > 0 && (
                            <div>
                              <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-2">Resources Touched</p>
                              <div className="flex flex-wrap gap-1">
                                {screen.touches_resources.map(t => (
                                  <span key={t} className="text-xs px-2 py-0.5 rounded bg-purple-500/20 text-purple-300 font-mono">{t}</span>
                                ))}
                              </div>
                            </div>
                          )}

                          <div className="border-t border-border-subtle pt-4">
                            <div className="flex items-center justify-between mb-2">
                              <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest">Cache</p>
                              <div className="flex gap-2">
                                <button
                                  onClick={() => flushScreenCache(screen.name)}
                                  disabled={cacheBusy === screen.name}
                                  className="px-3 py-1 rounded text-xs font-semibold bg-purple-500 hover:bg-purple-400 text-white disabled:opacity-60"
                                >
                                  {cacheBusy === screen.name ? '…' : 'Reset Screen Cache'}
                                </button>
                              </div>
                            </div>
                            {screen.sections.length > 0 && (
                              <div className="flex flex-wrap gap-2">
                                {screen.sections.map(sec => (
                                  <button
                                    key={sec.key}
                                    onClick={() => flushScreenCache(screen.name, sec.key)}
                                    disabled={cacheBusy === `${screen.name}/${sec.key}`}
                                    className="px-2 py-0.5 rounded text-[10px] font-mono bg-purple-500/20 text-purple-300 hover:bg-purple-500/30 disabled:opacity-60"
                                  >
                                    flush {sec.key}
                                  </button>
                                ))}
                              </div>
                            )}
                            {cacheMsg && <p className="text-[10px] text-text-muted mt-2">{cacheMsg}</p>}
                          </div>

                          <div className="border-t border-border-subtle pt-4">
                            <div className="flex items-center justify-between mb-2">
                              <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest">
                                Observe{observeMetrics?.provider?.display_name ? ` — ${observeMetrics.provider.display_name}` : ''}
                              </p>
                              {observeMetrics?.provider?.vendor_url && (
                                <a
                                  href={observeMetrics.provider.vendor_url}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="text-[10px] text-blue-400 hover:underline"
                                >
                                  View in {observeMetrics.provider.display_name || 'vendor'} ↗
                                </a>
                              )}
                            </div>
                            {observeLoading && <p className="text-xs text-text-muted">Loading metrics…</p>}
                            {!observeLoading && observeMetrics && (observeMetrics.samples?.length ?? 0) > 0 && (
                              <div className="grid grid-cols-4 gap-2 text-xs">
                                {(() => {
                                  const last = observeMetrics.samples![observeMetrics.samples!.length - 1];
                                  return (
                                    <>
                                      <InfoCell label="Traffic" value={String(last.traffic ?? '—')} />
                                      <InfoCell label="Latency (ms)" value={String(last.latency ?? '—')} />
                                      <InfoCell label="Errors" value={String(last.errors ?? '—')} />
                                      <InfoCell label="Users" value={String(last.users ?? '—')} />
                                    </>
                                  );
                                })()}
                              </div>
                            )}
                            {!observeLoading && (!observeMetrics || (observeMetrics.samples?.length ?? 0) === 0) && (
                              <p className="text-xs text-text-muted">No telemetry samples available. Per-screen instrumentation is a follow-up.</p>
                            )}
                          </div>

                          <div className="border-t border-border-subtle pt-4">
                            <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-2">Kill Switch</p>
                            <KillSwitchControl
                              screenName={screen.name}
                              current={screen.kill_switch}
                              onToggle={(enabled, reason, expiresIn) =>
                                toggleKillSwitch(screen.name, undefined, enabled, reason, expiresIn)
                              }
                            />
                            {screen.sections.length > 0 && (
                              <div className="mt-3 space-y-2">
                                <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest">Section Kill Switches</p>
                                {screen.sections.map(sec => {
                                  const ks = screen.section_kill_switches?.[sec.key];
                                  return (
                                    <div key={sec.key} className="rounded border border-border-subtle px-3 py-2">
                                      <p className="text-xs font-mono text-text-main mb-1.5">{sec.key}</p>
                                      <KillSwitchControl
                                        screenName={screen.name}
                                        current={ks}
                                        compact
                                        onToggle={(enabled, reason, expiresIn) =>
                                          toggleKillSwitch(screen.name, sec.key, enabled, reason, expiresIn)
                                        }
                                      />
                                    </div>
                                  );
                                })}
                              </div>
                            )}
                          </div>

                          <div className="border-t border-border-subtle pt-4">
                            <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest mb-2">Inspect — Run as user</p>
                            <div className="flex flex-wrap gap-2 items-center">
                              <input
                                type="text"
                                value={runAsUserId}
                                onChange={e => setRunAsUserId(e.target.value)}
                                placeholder="user_id (leave blank for anonymous)"
                                className="flex-1 min-w-[200px] bg-background border border-border-subtle rounded px-3 py-1.5 text-xs text-text-main outline-none focus:border-blue-400"
                              />
                              <input
                                type="text"
                                value={runAsLocale}
                                onChange={e => setRunAsLocale(e.target.value)}
                                placeholder="locale"
                                className="w-24 bg-background border border-border-subtle rounded px-3 py-1.5 text-xs text-text-main outline-none focus:border-blue-400"
                              />
                              <button
                                onClick={() => handleRunAs(screen.name)}
                                disabled={runAsLoading}
                                className="px-3 py-1.5 rounded text-xs font-semibold bg-blue-500 hover:bg-blue-400 text-white disabled:opacity-60"
                              >
                                {runAsLoading ? 'Running…' : 'Run'}
                              </button>
                            </div>
                            {runAsError && (
                              <div className="mt-3 rounded border border-red-500/40 bg-red-500/10 text-red-300 px-3 py-2 text-xs">
                                {runAsError}
                              </div>
                            )}
                            {runAsResult && (
                              <pre className="mt-3 rounded border border-border-subtle bg-background p-3 text-[11px] text-text-main overflow-auto max-h-96 font-mono">
                                {JSON.stringify(runAsResult, null, 2)}
                              </pre>
                            )}
                          </div>
                        </div>
                      );
                    })()}
                  </div>
                </div>
              </motion.div>
            )}

            {activePage === 'app-settings' && (
              <motion.div key="app-settings" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="flex flex-col h-full">
                <PageHeader title="App Configuration" description="Global mobile application parameters and behavioral overrides." />
                
                <div className="flex gap-12 flex-1 mt-4">
                   <div className="w-56 shrink-0 space-y-1">
                      {[
                        { id: 'general', label: 'Service Control', icon: Smartphone },
                        { id: 'onboarding', label: 'UX & Content', icon: Monitor },
                        { id: 'auth', label: 'Access & Auth', icon: ShieldCheck },
                      ].map(tab => (
                        <button 
                          key={tab.id}
                          onClick={() => setAppSubTab(tab.id as any)}
                          className={cn(
                            "w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all font-display text-[11px] font-bold uppercase tracking-widest text-left",
                            appSubTab === tab.id 
                              ? "bg-blue-500 text-black shadow-lg shadow-blue-500/20" 
                              : "text-[#8c90a1] hover:text-white hover:bg-surface-dim"
                          )}
                        >
                          <tab.icon size={16} /> <span>{tab.label}</span>
                        </button>
                      ))}
                   </div>

                   <div className="flex-1 space-y-8 animate-in fade-in slide-in-from-right-4 duration-500">
                      {appSubTab === 'general' && (
                        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
                           <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                              <h3 className="font-display text-lg font-semibold text-white mb-6">Service Continuity</h3>
                              <div className="space-y-6">
                                 {[
                                   { id: 'maintenance', label: 'Maintenance Mode', desc: 'Prevents users from accessing the app during updates.' },
                                   { id: 'force_update', label: 'Force Update', desc: 'Require users to update to the latest version.' },
                                   { id: 'push_notifs', label: 'Global Notifications', desc: 'Toggle all transactional and marketing push messages.' },
                                   { id: 'analytics', label: 'Internal Tracking', desc: 'Collect anonymous usage metrics for product refinement.' },
                                 ].map(item => {
                                   const isEnabled = settings[item.id] === 'true';
                                   return (
                                     <div key={item.id} className="flex justify-between items-start">
                                        <div className="max-w-[70%]">
                                           <p className="font-sans font-medium text-white">{item.label}</p>
                                           <p className="text-xs text-[#8c90a1] mt-1">{item.desc}</p>
                                        </div>
                                        <button 
                                          onClick={() => setSettings(prev => ({ ...prev, [item.id]: isEnabled ? 'false' : 'true' }))}
                                          className={cn("w-12 h-6 rounded-full relative transition-colors", isEnabled ? "bg-blue-500" : "bg-[#2D343C]")}
                                        >
                                           <div className={cn("absolute top-1 w-4 h-4 rounded-full bg-white transition-all", isEnabled ? "right-1" : "left-1")} />
                                        </button>
                                     </div>
                                   );
                                 })}
                              </div>
                           </section>
 
                           <div className="space-y-6">
                              <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                                 <h3 className="font-display text-lg font-semibold text-white mb-6">Device Support</h3>
                                 <div className="grid grid-cols-2 gap-4">
                                    <FormField label="Min iOS Build">
                                       <input 
                                         type="text" 
                                         value={settings.min_ios_build || ''} 
                                         onChange={e => setSettings(prev => ({ ...prev, min_ios_build: e.target.value }))}
                                         className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2 text-sm text-white" 
                                       />
                                    </FormField>
                                    <FormField label="Min Android SDK">
                                       <input 
                                         type="text" 
                                         value={settings.min_android_sdk || ''} 
                                         onChange={e => setSettings(prev => ({ ...prev, min_android_sdk: e.target.value }))}
                                         className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2 text-sm text-white" 
                                       />
                                    </FormField>
                                 </div>
                              </section>
                              
                              <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                                 <h3 className="font-display text-lg font-semibold text-white mb-6">Theme Overrides</h3>
                                 <div className="flex gap-3">
                                    {['Dark', 'Light', 'System'].map(t => {
                                       const isActive = settings.theme_override === t;
                                       return (
                                          <button 
                                            key={t} 
                                            onClick={() => setSettings(prev => ({ ...prev, theme_override: t }))}
                                            className={cn("flex-1 py-3 rounded-lg border text-[10px] font-bold uppercase tracking-widest transition-all", 
                                              isActive ? "bg-blue-500 text-black border-blue-500" : "border-border-subtle text-[#8c90a1] hover:text-white"
                                            )}
                                          >
                                            {t}
                                          </button>
                                       );
                                    })}
                                 </div>
                              </section>
                           </div>
                        </div>
                      )}

                      {appSubTab === 'onboarding' && (
                        <div className="max-w-4xl space-y-6">
                           <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                              <h3 className="font-display text-lg font-semibold text-white mb-6">Digital Experience</h3>
                              <div className="space-y-6">
                                 <div className="p-4 bg-surface-dim border border-blue-400/20 rounded-xl flex items-center justify-between">
                                    <div className="flex items-center gap-3">
                                       <CheckCircle2 size={18} className="text-blue-400" />
                                       <span className="text-sm font-medium text-white">Production Walkthrough Stable</span>
                                    </div>
                                    <button className="text-xs text-blue-400 hover:underline">Re-record Flow</button>
                                 </div>
                                 <div className="space-y-4">
                                    <FormField label="Welcome Sequence Copy">
                                       <textarea 
                                         className="w-full bg-background border border-border-subtle rounded-lg p-3 text-sm text-white h-32 resize-none" 
                                         value={settings.welcome_copy || ''} 
                                         onChange={e => setSettings(prev => ({ ...prev, welcome_copy: e.target.value }))}
                                       />
                                    </FormField>
                                    <div className="grid grid-cols-2 gap-6">
                                       <div className="space-y-4">
                                          <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest">Mobile Launcher Icon</p>
                                          <div className="w-20 h-20 rounded-2xl bg-surface-dim border border-dashed border-border-subtle flex flex-col items-center justify-center gap-2 group cursor-pointer hover:border-blue-400 transition-colors">
                                             <Smartphone size={24} className="text-[#424655] group-hover:text-blue-400" />
                                             <span className="text-[10px] text-[#424655]">1024x1024</span>
                                          </div>
                                       </div>
                                       <div className="space-y-4">
                                          <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest">Splash Screen Asset</p>
                                          <div className="w-full h-20 rounded-2xl bg-surface-dim border border-dashed border-border-subtle flex items-center justify-center gap-2 group cursor-pointer hover:border-blue-400 transition-colors">
                                             <Monitor size={20} className="text-[#424655] group-hover:text-blue-400" />
                                             <span className="text-[10px] text-[#424655]">Upload (PNG/SVG)</span>
                                          </div>
                                       </div>
                                    </div>
                                    <div className="grid grid-cols-2 gap-4">
                                       <FormField label="CTA Button Text">
                                          <input 
                                            type="text" 
                                            value={settings.cta_button_text || ''} 
                                            onChange={e => setSettings(prev => ({ ...prev, cta_button_text: e.target.value }))}
                                            className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2 text-sm text-white" 
                                          />
                                       </FormField>
                                       <FormField label="Accent Color (HEX)">
                                          <div className="flex gap-2">
                                             <div 
                                               className="w-10 h-10 rounded flex-shrink-0" 
                                               style={{ backgroundColor: settings.accent_color || '#3b82f6' }}
                                             />
                                             <input 
                                               type="text" 
                                               value={settings.accent_color || ''} 
                                               onChange={e => setSettings(prev => ({ ...prev, accent_color: e.target.value }))}
                                               className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2 text-sm text-white font-mono" 
                                             />
                                          </div>
                                       </FormField>
                                    </div>
                                 </div>
                              </div>
                           </section>
                        </div>
                      )}

                      {appSubTab === 'auth' && (
                        <div className="max-w-3xl space-y-6">
                           <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                              <h3 className="font-display text-lg font-semibold text-white mb-6">Access Policies</h3>
                              <div className="space-y-6">
                                 <div className="flex justify-between items-center p-4 bg-surface-dim rounded-xl border border-red-500/20">
                                    <div>
                                       <p className="font-sans font-medium text-white">Disable Guest Sign-up</p>
                                       <p className="text-[10px] text-red-400 uppercase tracking-widest font-bold mt-1">Experimental</p>
                                    </div>
                                    <button 
                                      onClick={() => setSettings(prev => ({ ...prev, disable_guest_signup: prev.disable_guest_signup === 'true' ? 'false' : 'true' }))}
                                      className={cn("w-12 h-6 rounded-full relative transition-colors", settings.disable_guest_signup === 'true' ? "bg-red-500" : "bg-[#2D343C]")}
                                    >
                                       <div className={cn("absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all", settings.disable_guest_signup === 'true' ? "right-0.5" : "left-0.5")} />
                                    </button>
                                 </div>
                                 <div className="flex justify-between items-center p-4 bg-surface-dim rounded-xl border border-border-subtle">
                                    <div>
                                       <p className="font-sans font-medium text-white">Biometric Enforcement</p>
                                       <p className="text-xs text-[#8c90a1] mt-1">Require FaceID/Fingerprint for high-value transactions.</p>
                                    </div>
                                    <button 
                                      onClick={() => setSettings(prev => ({ ...prev, biometric_enforcement: prev.biometric_enforcement === 'true' ? 'false' : 'true' }))}
                                      className={cn("w-12 h-6 rounded-full relative transition-colors", settings.biometric_enforcement === 'true' ? "bg-blue-500" : "bg-[#2D343C]")}
                                    >
                                       <div className={cn("absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all", settings.biometric_enforcement === 'true' ? "right-0.5" : "left-0.5")} />
                                    </button>
                                 </div>
                              </div>
                           </section>

                           <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                              <h3 className="font-display text-lg font-semibold text-white mb-6">SSO Integration</h3>
                              <div className="flex gap-4">
                                 {['Google', 'Apple', 'Github'].map(p => (
                                    <div key={p} className="flex-1 p-4 bg-background border border-border-subtle rounded-xl flex flex-col items-center gap-3">
                                       <div className="w-8 h-8 rounded-full bg-surface-dim border border-border-subtle" />
                                       <span className="text-[10px] font-bold uppercase tracking-widest text-[#8c90a1]">{p}</span>
                                       <div className="w-10 h-1.5 bg-blue-500 rounded-full" />
                                    </div>
                                 ))}
                              </div>
                           </section>
                        </div>
                      )}

                      <div className="flex justify-start pt-4">
                         <button 
                           onClick={saveSettings}
                           className="px-12 py-4 bg-blue-500 text-black font-bold rounded-2xl shadow-2xl shadow-blue-500/20 flex items-center gap-3 hover:bg-blue-400 transition-all hover:scale-[1.02] active:scale-[0.98]"
                         >
                           <Save size={20} /> Deploy Global Configuration
                         </button>
                      </div>
                   </div>
                </div>
              </motion.div>
            )}

            {activePage === 'system-settings' && (
              <motion.div key="system-settings" initial={{ opacity: 0, scale: 0.98 }} animate={{ opacity: 1, scale: 1 }} className="flex flex-col h-full">
                <PageHeader title="Infrastructure Control" description="Manage engine variables, keys, and backend endpoints." />
                
                <div className="flex gap-12 flex-1 mt-4">
                   <div className="w-56 shrink-0 space-y-1">
                      {[
                        { id: 'keys', label: 'API Keys', icon: Key },
                        { id: 'admins', label: 'Admin Access', icon: Lock },
                        { id: 'workers', label: 'Worker Nodes', icon: Cpu },
                        { id: 'jobs', label: 'Background Jobs', icon: Activity },
                        { id: 'security', label: 'Hardening', icon: ShieldCheck },
                      ].map(tab => (
                        <button 
                          key={tab.id}
                          onClick={() => setSystemSubTab(tab.id as any)}
                          className={cn(
                            "w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all font-display text-[11px] font-bold uppercase tracking-widest text-left",
                            systemSubTab === tab.id 
                              ? "bg-blue-500 text-black shadow-lg shadow-blue-500/20" 
                              : "text-[#8c90a1] hover:text-white hover:bg-surface-dim"
                          )}
                        >
                          <tab.icon size={16} /> <span>{tab.label}</span>
                        </button>
                      ))}
                   </div>

                   <div className="flex-1 space-y-8 animate-in fade-in slide-in-from-right-4 duration-500">
                      {systemSubTab === 'keys' && (
                        <section className="bg-surface-card border border-border-subtle rounded-xl overflow-hidden shadow-2xl">
                           <div className="p-6 border-b border-border-subtle bg-surface-dim flex justify-between items-center">
                              <h3 className="font-display text-lg font-semibold text-white flex items-center gap-3">
                                 <Key size={20} className="text-blue-400" /> API Access Keys
                              </h3>
                              <button className="px-3 py-1.5 bg-blue-500 text-black font-bold rounded-lg text-[10px] uppercase tracking-widest flex items-center gap-2">
                                 <Plus size={14} /> Generate Key
                              </button>
                           </div>
                           <div className="overflow-x-auto">
                              <table className="w-full text-left">
                                 <thead className="bg-[#111416]/50 font-display text-[10px] font-bold text-[#424655] uppercase tracking-widest">
                                    <tr>
                                       <th className="py-4 px-8">Client Name</th>
                                       <th className="py-4 px-8">Key Value</th>
                                       <th className="py-4 px-8">Created</th>
                                       <th className="py-4 px-8 text-right">Revoke</th>
                                    </tr>
                                 </thead>
                                 <tbody className="font-sans text-sm text-white divide-y divide-border-subtle/50">
                                    {[
                                      { name: 'Production Mobile App', key: 'obs_live_9a2f...1e4d', date: 'Oct 12, 2023' },
                                      { name: 'Staging Environment', key: 'obs_test_88c1...99x3', date: 'Jan 05, 2024' },
                                      { name: 'Analytic Exporter Service', key: 'obs_ext_11z0...aa7c', date: 'Mar 18, 2024' },
                                    ].map(k => (
                                      <tr key={k.key} className="hover:bg-surface-dim transition-colors group">
                                         <td className="py-4 px-8 font-medium">{k.name}</td>
                                         <td className="py-4 px-8"><code className="font-mono text-xs px-2 py-1 bg-background rounded text-[#8c90a1] border border-border-subtle">{k.key}</code></td>
                                         <td className="py-4 px-8 text-[#8c90a1] text-xs font-mono">{k.date}</td>
                                         <td className="py-4 px-8 text-right">
                                            <button className="text-red-400 opacity-0 group-hover:opacity-100 transition-opacity hover:underline">Revoke</button>
                                         </td>
                                      </tr>
                                    ))}
                                 </tbody>
                              </table>
                           </div>
                        </section>
                      )}

                      {systemSubTab === 'admins' && (
                        <div className="bg-surface-card border border-border-subtle rounded-xl overflow-hidden shadow-2xl">
                           <div className="p-6 border-b border-border-subtle bg-surface-dim flex justify-between items-center">
                              <h3 className="font-display text-lg font-semibold text-white flex items-center gap-3">
                                 <Lock size={20} className="text-blue-400" /> Admin Access Control
                              </h3>
                              <button 
                                onClick={() => { setEditingAdmin(null); setFormData({}); setIsAdminModalOpen(true); }}
                                className="px-4 py-2 bg-blue-500 text-black font-bold rounded-lg text-sm flex items-center gap-2"
                              >
                                <ShieldCheck size={18} /> Grant Privilege
                              </button>
                           </div>
                           <div className="overflow-x-auto">
                             <table className="w-full text-left">
                               <thead className="bg-[#111416] border-b border-border-subtle font-display text-[10px] font-bold text-[#9BA3AF] uppercase tracking-widest">
                                 <tr>
                                   <th className="py-4 px-6">Administrator</th>
                                   <th className="py-4 px-6">Tier</th>
                                   <th className="py-4 px-6">Session</th>
                                   <th className="py-4 px-6 text-right">Actions</th>
                                 </tr>
                               </thead>
                               <tbody className="font-sans text-sm text-white divide-y divide-border-subtle">
                                 {adminUsers.map((adm) => (
                                   <tr key={adm.id} className="hover:bg-surface-dim group transition-colors">
                                     <td className="py-4 px-6">
                                       <div className="flex items-center gap-3">
                                         <div className="w-8 h-8 rounded bg-surface-dim border border-blue-400/30 flex items-center justify-center"><Lock size={14} className="text-blue-400" /></div>
                                         <div>
                                           <p className="font-bold">{adm.name}</p>
                                           <p className="text-[10px] text-[#8c90a1]">{adm.email}</p>
                                         </div>
                                       </div>
                                     </td>
                                     <td className="py-4 px-6">
                                        <span className="font-mono text-xs px-2 py-0.5 bg-blue-400/10 text-blue-400 rounded border border-blue-400/20">{adm.tier}</span>
                                     </td>
                                     <td className="py-4 px-6 text-[#8c90a1] font-mono text-xs">{adm.time}</td>
                                     <td className="py-4 px-6 text-right">
                                        <div className="flex justify-end gap-2 text-[#8c90a1] opacity-0 group-hover:opacity-100 transition-opacity">
                                           <button onClick={() => handleEditAdmin(adm)} className="p-1.5 hover:text-white transition-colors">Edit</button>
                                           <button onClick={() => handleDeleteAdmin(adm)} className="p-1.5 text-red-400 hover:underline">Revoke</button>
                                        </div>
                                     </td>
                                   </tr>
                                 ))}
                               </tbody>
                             </table>
                           </div>
                        </div>
                      )}

                      {systemSubTab === 'workers' && (
                         <div className="bg-surface-card border border-border-subtle rounded-xl overflow-hidden shadow-2xl">
                            <div className="p-6 border-b border-border-subtle bg-surface-dim">
                               <h3 className="font-display text-lg font-semibold text-white flex items-center gap-3">
                                  <Cpu size={20} className="text-blue-400" /> Worker Configuration
                               </h3>
                            </div>
                            <div className="p-8">
                               <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
                                  <div className="lg:col-span-1 border-r border-border-subtle space-y-2 pr-8">
                                     {['Email Delivery', 'Image Processing', 'Log Aggregator', 'Push Dispatcher'].map((w, i) => (
                                        <button 
                                           key={i} 
                                           className={cn(
                                              "w-full text-left px-4 py-3 rounded-xl font-display text-[10px] font-bold uppercase tracking-widest transition-all",
                                              i === 0 ? "bg-blue-500 text-black shadow-lg shadow-blue-500/20" : "text-[#8c90a1] hover:text-white hover:bg-surface-dim"
                                           )}
                                        >
                                           {w}
                                        </button>
                                     ))}
                                     <button className="w-full text-center px-4 py-3 rounded-xl border border-dashed border-border-subtle text-[#424655] hover:text-blue-400 hover:border-blue-400 transition-all text-[10px] font-bold uppercase mt-4">+ Register Worker</button>
                                  </div>
                                  <div className="lg:col-span-3 space-y-6">
                                     <div className="grid grid-cols-2 gap-6">
                                        <FormField label="Endpoint URL">
                                           <input type="text" defaultValue="https://workers.obsidian.io/v1/mail" className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white" />
                                        </FormField>
                                        <FormField label="Secret Token">
                                           <input type="password" defaultValue="***************" className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white" />
                                        </FormField>
                                        <FormField label="Concurrency Level">
                                           <input type="number" defaultValue="4" className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white" />
                                        </FormField>
                                        <FormField label="Retry Strategy">
                                           <select className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white">
                                              <option>Exponential Backoff</option>
                                              <option>Linear Wait</option>
                                              <option>Immediate Fail</option>
                                           </select>
                                        </FormField>
                                     </div>
                                     <div className="pt-6 border-t border-border-subtle flex justify-end">
                                        <button className="px-6 py-3 bg-blue-500 text-black font-bold rounded-lg text-xs uppercase tracking-widest flex items-center gap-2">
                                           <Save size={14} /> Update Worker
                                        </button>
                                     </div>
                                  </div>
                               </div>
                            </div>
                         </div>
                      )}

                      {systemSubTab === 'jobs' && (
                        <div className="bg-surface-card border border-border-subtle rounded-xl overflow-hidden shadow-2xl">
                           <div className="p-6 border-b border-border-subtle bg-surface-dim flex justify-between items-center">
                              <h3 className="font-display text-lg font-semibold text-white flex items-center gap-3">
                                 <Activity size={20} className="text-blue-400" /> Background Execution Jobs
                              </h3>
                              <button 
                                onClick={loadJobs}
                                className="px-3 py-1.5 bg-blue-500 text-black font-bold rounded-lg text-[10px] uppercase tracking-widest flex items-center gap-2 hover:bg-blue-400 transition-colors"
                              >
                                 <RefreshCw size={14} className={isLoadingJobs ? "animate-spin" : ""} /> Refresh
                              </button>
                           </div>
                           <div className="overflow-x-auto">
                              <table className="w-full text-left">
                                 <thead className="bg-[#111416]/50 font-display text-[10px] font-bold text-[#424655] uppercase tracking-widest">
                                    <tr>
                                       <th className="py-4 px-8">Job ID</th>
                                       <th className="py-4 px-8">User ID</th>
                                       <th className="py-4 px-8">Kind & Name</th>
                                       <th className="py-4 px-8">Status</th>
                                       <th className="py-4 px-8">Result / Error</th>
                                       <th className="py-4 px-8">Created</th>
                                       <th className="py-4 px-8 text-right">Actions</th>
                                    </tr>
                                 </thead>
                                 <tbody className="font-sans text-sm text-white divide-y divide-border-subtle/50">
                                    {jobs.length === 0 ? (
                                       <tr>
                                          <td colSpan={7} className="py-8 text-center text-[#8c90a1]">
                                             {isLoadingJobs ? "Loading background jobs..." : "No background jobs registered."}
                                          </td>
                                       </tr>
                                    ) : (
                                       jobs.map(job => (
                                          <tr key={job.id} className="hover:bg-surface-dim transition-colors group">
                                             <td className="py-4 px-8 font-mono text-xs text-[#8c90a1]">{job.id.substring(0, 8)}...</td>
                                             <td className="py-4 px-8 text-xs">{job.user_id || 'system'}</td>
                                             <td className="py-4 px-8">
                                                <div className="flex flex-col">
                                                   <span className="font-semibold text-white">{job.name}</span>
                                                   <span className="text-[10px] text-[#8c90a1] uppercase font-mono tracking-wider">{job.kind}</span>
                                                </div>
                                             </td>
                                             <td className="py-4 px-8">
                                                <span className={cn(
                                                   "px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider font-mono",
                                                   job.status === 'done' && "bg-green-500/10 text-green-400 border border-green-500/20",
                                                   job.status === 'failed' && "bg-red-500/10 text-red-400 border border-red-500/20",
                                                   job.status === 'running' && "bg-blue-500/10 text-blue-400 border border-blue-500/20 animate-pulse",
                                                   job.status === 'pending' && "bg-yellow-500/10 text-yellow-400 border border-yellow-500/20"
                                                )}>
                                                   {job.status}
                                                </span>
                                             </td>
                                             <td className="py-4 px-8 max-w-[200px] truncate">
                                                {job.error ? (
                                                   <span className="text-red-400 text-xs font-mono">{job.error}</span>
                                                ) : job.result ? (
                                                   <code className="text-[#8c90a1] text-xs font-mono">{JSON.stringify(job.result)}</code>
                                                ) : (
                                                   <span className="text-[#424655] font-mono text-xs">-</span>
                                                )}
                                             </td>
                                             <td className="py-4 px-8 text-[#8c90a1] text-xs font-mono">
                                                {job.created_at ? new Date(job.created_at).toLocaleString() : ''}
                                             </td>
                                             <td className="py-4 px-8 text-right">
                                                <div className="flex justify-end gap-2">
                                                   {job.status === 'failed' && (
                                                      <button 
                                                         onClick={() => handleRetryJob(job.id)}
                                                         title="Retry Job"
                                                         className="p-1.5 bg-green-500/10 hover:bg-green-500 hover:text-black border border-green-500/20 rounded text-green-400 transition-all flex items-center justify-center"
                                                      >
                                                         <Play size={12} fill="currentColor" />
                                                      </button>
                                                   )}
                                                   {(job.status === 'pending' || job.status === 'running') && (
                                                      <button 
                                                         onClick={() => handleCancelJob(job.id)}
                                                         title="Cancel Job"
                                                         className="p-1.5 bg-red-500/10 hover:bg-red-500 hover:text-white border border-red-500/20 rounded text-red-400 transition-all flex items-center justify-center"
                                                      >
                                                         <Square size={12} fill="currentColor" />
                                                      </button>
                                                   )}
                                                </div>
                                             </td>
                                          </tr>
                                       ))
                                    )}
                                 </tbody>
                              </table>
                           </div>
                        </div>
                      )}

                      {systemSubTab === 'security' && (
                        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
                           <section className="bg-surface-card border border-border-subtle rounded-xl p-8">
                              <h3 className="font-display text-lg font-semibold text-white mb-6">IP Management</h3>
                              <div className="space-y-4">
                                 <p className="text-xs text-[#8c90a1] mb-4 leading-relaxed">Restrict administrative access to specific IP addresses for increased security hardening.</p>
                                 <div className="flex flex-wrap gap-2 mb-4">
                                    {['192.168.1.1', '10.0.0.12', '72.14.99.102'].map(ip => (
                                       <span key={ip} className="px-2 py-1 bg-surface-dim border border-border-subtle rounded font-mono text-xs text-white flex items-center gap-2">
                                          {ip} <X size={10} className="text-[#424655] cursor-pointer hover:text-red-400" />
                                       </span>
                                    ))}
                                 </div>
                                 <div className="flex gap-2">
                                    <input type="text" placeholder="Enter IP address..." className="flex-1 bg-background border border-border-subtle rounded-lg px-4 py-2 text-sm text-white font-mono" />
                                    <button className="px-4 py-2 bg-blue-500 text-black font-bold rounded-lg text-xs uppercase tracking-widest">Whitelist</button>
                                 </div>
                                 <button className="w-full mt-4 py-3 border border-red-500/30 text-red-400 hover:bg-red-500 hover:text-white rounded-lg text-xs font-bold uppercase tracking-widest transition-all flex items-center justify-center gap-2">
                                    <ShieldAlert size={14} /> Remove All Whitelisted IPs
                                 </button>
                              </div>
                           </section>

                           <div className="space-y-6">
                              <section className="bg-gradient-to-br from-blue-500/10 to-transparent border border-blue-400/20 rounded-xl p-8">
                                 <div className="flex items-center gap-3 text-blue-400 mb-4">
                                    <ShieldCheck size={24} />
                                    <h4 className="font-display font-bold text-lg">Platform Hardening</h4>
                                 </div>
                                 <div className="space-y-4">
                                    <div className="flex justify-between items-center">
                                       <span className="text-sm text-white">TLS 1.3 Strict Mode</span>
                                       <div className="w-10 h-5 rounded-full bg-blue-500 relative"><div className="absolute top-0.5 right-0.5 w-4 h-4 rounded-full bg-white" /></div>
                                    </div>
                                    <div className="flex justify-between items-center">
                                       <span className="text-sm text-[#8c90a1]">Rate Limiting (Shield)</span>
                                       <div className="w-10 h-5 rounded-full bg-blue-500 relative"><div className="absolute top-0.5 right-0.5 w-4 h-4 rounded-full bg-white" /></div>
                                    </div>
                                 </div>
                              </section>
                              
                              <button className="w-full py-4 bg-surface-dim border border-border-subtle hover:border-red-400/50 text-[#8c90a1] hover:text-red-400 rounded-xl text-xs font-bold uppercase tracking-widest transition-all flex items-center justify-center gap-2">
                                 <RefreshCw size={16} /> Clear System Cache
                              </button>
                           </div>
                        </div>
                      )}
                   </div>
                </div>
               </motion.div>
             )}

             {activePage === 'api-docs' && (
               <motion.div key="api-docs" initial={{ opacity: 0, scale: 0.98 }} animate={{ opacity: 1, scale: 1 }} className="flex flex-col h-full">
                 <PageHeader title="API Reference" description="Interactive OpenAPI explorer for this project (development only)." />
                 <div className="flex-1 mt-4 rounded-2xl overflow-hidden bg-white border border-border-subtle min-h-[480px]">
                   <ApiDocsPanel
                     assetBase={API_BASE}
                     specUrl={`${ADMIN_API}/docs/openapi.json`}
                   />
                 </div>
               </motion.div>
             )}

             {activePage === 'bffx-docs' && (
               <motion.div key="bffx-docs" initial={{ opacity: 0, scale: 0.98 }} animate={{ opacity: 1, scale: 1 }} className="flex flex-col h-full">
                 <PageHeader
                   title="BFFX Guide"
                   description="Bundled framework documentation from .bffx/docs/ (development only)."
                 />
                 <BffxDocsPanel adminApiUrl={ADMIN_API} />
               </motion.div>
             )}
           </AnimatePresence>
         </div>

        {/* Modal: Admin Create/Edit */}
        <Modal 
          isOpen={isAdminModalOpen} 
          onClose={() => setIsAdminModalOpen(false)} 
          title={editingAdmin ? 'Modify Access Tier' : 'Grant Administrative Access'}
        >
          <div className="space-y-4">
            <FormField label="Staff Name">
              <input 
                type="text" 
                value={formData.name || ''} 
                onChange={(e) => setFormData({...formData, name: e.target.value})}
                className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none" 
              />
            </FormField>
            <FormField label="Internal Email">
              <input 
                type="email" 
                value={formData.email || ''} 
                onChange={(e) => setFormData({...formData, email: e.target.value})}
                className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none" 
              />
            </FormField>
            <FormField label="Permission Tier">
              <select 
                value={formData.tier || ''} 
                onChange={(e) => setFormData({...formData, tier: e.target.value})}
                className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none"
              >
                <option value="">Select Tier</option>
                <option value="Superuser">Superuser</option>
                <option value="Full Control">Full Control</option>
                <option value="Security">Security</option>
                <option value="Developer">Developer</option>
                <option value="Auditor">Auditor</option>
              </select>
            </FormField>
            <div className="pt-4 flex gap-3">
              <button onClick={() => setIsAdminModalOpen(false)} className="flex-1 px-4 py-3 border border-border-subtle rounded-lg text-sm font-bold text-white hover:bg-surface-dim transition-all">Cancel</button>
              <button onClick={saveAdmin} className="flex-1 px-4 py-3 bg-blue-500 text-black font-bold rounded-lg text-sm hover:bg-blue-400 transition-all">Update Privileges</button>
            </div>
          </div>
        </Modal>

        {/* Modal: Resource Create/Edit */}
        <Modal 
          isOpen={isResourceModalOpen} 
          onClose={() => setIsResourceModalOpen(false)} 
          title={editingResource ? `Edit ${humanizeResourceName(activeCollection.name, activeCollection.displayName)}` : `New ${humanizeResourceName(activeCollection.name, activeCollection.displayName)} Entry`}
        >
          <div className="space-y-4">
             {(activeCollection.spec?.form?.inputs || (activeCollection.fields || [])
               .filter(f => f.name !== 'id' && f.name !== 'created_at' && f.name !== 'updated_at' && f.name !== 'created_by')
               .map(f => ({ field: f.name, as: f.type }))
             ).map(input => {
               const fieldDef = (activeCollection.fields || []).find(f => f.name === input.field);
               const inputAs = input.as || fieldDef?.type || 'string';
               const targetResource = fieldDef?.target;

               if (inputAs === 'password' && editingResource) {
                 return null;
               }

               return (
                 <div key={input.field} className="space-y-1.5 flex-1">
                   <FormField label={input.field}>
                     {(() => {
                       if (inputAs === 'relation' && targetResource) {
                         return (
                           <RelationPicker
                             adminApiUrl={ADMIN_API}
                             targetResource={targetResource}
                             value={formData[input.field] || ''}
                             onChange={(val) => setFormData({ ...formData, [input.field]: val })}
                           />
                         );
                       }
                       if (inputAs === 'select' || inputAs === 'dropdown') {
                         return (
                           <select
                             value={formData[input.field] || ''}
                             onChange={(e) => setFormData({ ...formData, [input.field]: e.target.value })}
                             className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none"
                           >
                             <option value="">Select option...</option>
                             {(input.options || []).map((opt: string) => (
                               <option key={opt} value={opt}>{opt}</option>
                             ))}
                           </select>
                         );
                       }
                       if (inputAs === 'bool' || inputAs === 'boolean') {
                         return (
                           <select
                             value={formData[input.field] === undefined ? '' : String(formData[input.field])}
                             onChange={(e) => {
                               const val = e.target.value;
                               setFormData({
                                 ...formData,
                                 [input.field]: val === 'true' ? true : val === 'false' ? false : undefined
                               });
                             }}
                             className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none"
                           >
                             <option value="">Select...</option>
                             <option value="true">True</option>
                             <option value="false">False</option>
                           </select>
                         );
                       }
                       if (inputAs === 'date' || inputAs === 'datetime') {
                         return (
                           <input
                             type={inputAs === 'datetime' ? 'datetime-local' : 'date'}
                             value={formData[input.field] ? String(formData[input.field]).substring(0, 16) : ''}
                             onChange={(e) => setFormData({ ...formData, [input.field]: e.target.value })}
                             className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none"
                           />
                         );
                       }
                       if (inputAs === 'json') {
                         return (
                           <textarea
                             value={typeof formData[input.field] === 'object' ? JSON.stringify(formData[input.field], null, 2) : formData[input.field] || ''}
                             onChange={(e) => {
                               const val = e.target.value;
                               try {
                                 const parsed = JSON.parse(val);
                                 setFormData({ ...formData, [input.field]: parsed });
                               } catch (err) {
                                 setFormData({ ...formData, [input.field]: val });
                               }
                             }}
                             rows={4}
                             className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-xs font-mono text-white focus:border-blue-400 outline-none"
                             placeholder="{}"
                           />
                         );
                       }
                       if (inputAs === 'password') {
                         return (
                           <input
                             type="password"
                             value={formData[input.field] || ''}
                             onChange={(e) => setFormData({ ...formData, [input.field]: e.target.value })}
                             className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none"
                             placeholder="Enter password..."
                           />
                         );
                       }
                       return (
                         <input 
                            type={inputAs === 'email' ? 'email' : 'text'}
                            value={formData[input.field] || ''}
                            onChange={(e) => setFormData({ ...formData, [input.field]: e.target.value })}
                            className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none"
                         />
                       );
                     })()}
                   </FormField>
                 </div>
               );
             })}
             <div className="pt-4 flex gap-3">
              <button onClick={() => setIsResourceModalOpen(false)} className="flex-1 px-4 py-3 border border-border-subtle rounded-lg text-sm font-bold text-white hover:bg-surface-dim transition-all">Cancel</button>
              <button onClick={saveResource} className="flex-1 px-4 py-3 bg-blue-500 text-black font-bold rounded-lg text-sm hover:bg-blue-400 transition-all">Save Record</button>
            </div>
          </div>
        </Modal>

        {/* Modal: Resource Show/Detail */}
        <Modal 
          isOpen={isShowModalOpen} 
          onClose={() => { setIsShowModalOpen(false); setShowingResource(null); }} 
          title={`${humanizeResourceName(activeCollection.name, activeCollection.displayName)} Details`}
        >
          {showingResource && (
            <div className="space-y-4">
              <div className="border border-border-subtle rounded-lg overflow-hidden bg-background divide-y divide-border-subtle">
                {(activeCollection.fields || []).filter(field => {
                  const isExcluded = activeCollection.spec?.show?.exclude?.some((excl: string) => excl.toLowerCase() === field.name.toLowerCase());
                  return !isExcluded;
                }).map(field => {
                  const val = showingResource[field.name];
                  const belongs = (activeCollection.spec?.belongs_to || []).find(
                    (b: { field: string }) => b.field === field.name,
                  );
                  const fieldDef = (activeCollection.fields || []).find(
                    (fd: { name: string }) => fd.name === field.name,
                  );
                  const relTarget = belongs?.resource || fieldDef?.target;
                  return (
                    <div key={field.name} className="p-4 grid grid-cols-3 gap-4 hover:bg-surface-dim transition-colors">
                      <span className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-wider self-center">{humanizeResourceName(field.name)}</span>
                      <span className="col-span-2 text-sm text-white font-sans break-all">
                        {relTarget && val != null && val !== '' ? (
                          <RelationLink
                            targetResource={relTarget}
                            recordId={String(val)}
                            onOpen={openRelatedResource}
                          />
                        ) : typeof val === 'object' && val !== null ? (
                          <pre className="font-mono text-xs bg-[#0d0e10] p-2 rounded text-[#8c90a1] border border-border-subtle max-h-48 overflow-y-auto">
                            {JSON.stringify(val, null, 2)}
                          </pre>
                        ) : typeof val === 'boolean' ? (
                          <span className={cn("px-2 py-0.5 text-xs font-bold uppercase rounded", val ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400")}>
                            {val ? 'true' : 'false'}
                          </span>
                        ) : (
                          String(val ?? '-')
                        )}
                      </span>
                    </div>
                  );
                })}
              </div>
              {(activeCollection.spec?.associations || []).map((assoc: { name: string; resource: string; label?: string }) => (
                <AssociationPanel
                  key={assoc.name}
                  adminApiUrl={ADMIN_API}
                  parentResource={activeCollection.name}
                  parentId={String(showingResource.id)}
                  name={assoc.name}
                  label={assoc.label || assoc.name}
                  childResource={assoc.resource}
                />
              ))}
              <div className="pt-2 flex gap-3 flex-wrap">
                {(activeCollection.spec?.member_actions || []).map((act: any) => (
                  <button
                    key={act.name}
                    onClick={() => {
                      handleMemberAction(act, showingResource.id);
                      setIsShowModalOpen(false);
                      setShowingResource(null);
                    }}
                    className="flex-1 px-4 py-3 bg-[#111416] border border-border-subtle text-white font-bold rounded-lg text-sm hover:border-blue-400 hover:text-blue-400 transition-all whitespace-nowrap"
                  >
                    {act.label || act.name}
                  </button>
                ))}
                <button onClick={() => { setIsShowModalOpen(false); setShowingResource(null); }} className="flex-1 px-4 py-3 border border-border-subtle rounded-lg text-sm font-bold text-white hover:bg-surface-dim transition-all">Close</button>
              </div>
            </div>
          )}
        </Modal>

        {/* Modal: User Create/Edit */}
        <Modal 
          isOpen={isUserModalOpen} 
          onClose={() => setIsUserModalOpen(false)} 
          title={editingUser ? 'Edit Member Profile' : 'Register New Member'}
        >
          <div className="space-y-4">
            <FormField label="Full Name">
              <input 
                type="text" 
                value={formData.name || ''} 
                onChange={(e) => setFormData({...formData, name: e.target.value})} 
                className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none" 
              />
            </FormField>
            <FormField label="Email Address">
              <input 
                type="email" 
                value={formData.email || ''} 
                onChange={(e) => setFormData({...formData, email: e.target.value})} 
                className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none" 
              />
            </FormField>
            <div className="flex gap-4">
              <FormField label="Role">
                <select 
                  value={formData.role || 'User'} 
                  onChange={(e) => setFormData({...formData, role: e.target.value})}
                  className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none appearance-none"
                >
                  <option value="User">User</option>
                  <option value="Admin">Admin</option>
                  <option value="Moderator">Moderator</option>
                </select>
              </FormField>
              <FormField label="Account Status">
                <select 
                  value={formData.status || 'Active'} 
                  onChange={(e) => setFormData({...formData, status: e.target.value})}
                  className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none appearance-none"
                >
                  <option value="Active">Active</option>
                  <option value="Inactive">Inactive</option>
                  <option value="Pending">Pending</option>
                </select>
              </FormField>
            </div>
            <div className="pt-4 flex gap-3">
              <button onClick={() => setIsUserModalOpen(false)} className="flex-1 px-4 py-3 border border-border-subtle rounded-lg text-sm font-bold text-white hover:bg-surface-dim transition-all">Cancel</button>
              <button onClick={saveUser} className="flex-1 px-4 py-3 bg-blue-500 text-black font-bold rounded-lg text-sm hover:bg-blue-400 transition-all">Save Member</button>
            </div>
          </div>
        </Modal>

        {/* Modal: Feature Flag Create/Edit */}
        <Modal 
          isOpen={isFlagModalOpen} 
          onClose={() => setIsFlagModalOpen(false)} 
          title={editingFlag ? 'Modify Feature Toggle' : 'Initialize Feature Toggle'}
        >
          <div className="space-y-4">
            <FormField label="Flag Key (System ID)">
               <input 
                type="text" 
                value={formData.key || ''} 
                onChange={(e) => setFormData({...formData, key: e.target.value})} 
                placeholder="e.g., enable_experimental_layout"
                className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm font-mono text-blue-400 focus:border-blue-400 outline-none" 
              />
            </FormField>
            <FormField label="Objective description">
              <textarea 
                rows={3}
                value={formData.description || ''} 
                onChange={(e) => setFormData({...formData, description: e.target.value})} 
                className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none resize-none" 
              />
            </FormField>
            <FormField label="Environment">
              <select 
                value={formData.environment || 'Production'} 
                onChange={(e) => setFormData({...formData, environment: e.target.value})}
                className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none"
              >
                <option value="Production">Production</option>
                <option value="Staging">Staging</option>
                <option value="Development">Development</option>
              </select>
            </FormField>
            <div className="pt-4 flex gap-3">
              <button onClick={() => setIsFlagModalOpen(false)} className="flex-1 px-4 py-3 border border-border-subtle rounded-lg text-sm font-bold text-white hover:bg-surface-dim transition-all">Cancel</button>
              <button onClick={saveFlag} className="flex-1 px-4 py-3 bg-blue-500 text-black font-bold rounded-lg text-sm hover:bg-blue-400 transition-all">Implement Toggle</button>
            </div>
          </div>
        </Modal>

      </main>
    </div>
  );
}
