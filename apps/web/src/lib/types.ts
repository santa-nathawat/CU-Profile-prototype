// Shapes from docs/api.md. Add new sections here as screens need them.

export type Page<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
};

export type Rating = { average: number | null; count: number };
export type NamedRef = { id: string; name: string };
export type PersonRef = { id: string; displayName: string };

export type UserRef = {
  id: string;
  displayName: string;
  faculty: string | null;
  year: number | null;
};

export type ProjectStatus =
  'not_started' | 'in_progress' | 'completed' | 'cancelled';
export type RecruitmentStatus = 'open' | 'closed';
export type DurationUnit = 'day' | 'week' | 'month';

export type Role = {
  id: string;
  name: string;
  capacity: number;
  taken: number;
  available: number;
};

export type Member = {
  userId: string;
  displayName: string;
  faculty: string | null;
  roleId: string;
  roleName: string;
  joinedAt: string;
  isLead: boolean;
};

export type ProjectCard = {
  id: string;
  name: string;
  description: string;
  category: NamedRef;
  status: ProjectStatus;
  recruitmentStatus: RecruitmentStatus;
  durationValue: number;
  durationUnit: DurationUnit;
  placesTaken: number;
  placesTotal: number;
  lead: PersonRef & { rating: Rating };
  roles: Role[];
};

export type RequestViewer = {
  isStudent: boolean;
  isLead: boolean;
  canApprove: boolean;
  canReject: boolean;
  canWithdraw: boolean;
  canAccept: boolean;
  canDecline: boolean;
  canCancel: boolean;
};

export type RequestStatus =
  | 'pending'
  | 'approved'
  | 'rejected'
  | 'accepted'
  | 'declined'
  | 'withdrawn'
  | 'cancelled'
  | 'expired';

export type CancelReason =
  | 'lead_cancelled'
  | 'role_removed'
  | 'role_full'
  | 'recruitment_closed'
  | 'project_completed'
  | 'project_cancelled'
  | 'team_deleted'
  | 'account_closed';

export type RecruitmentRequest = {
  id: string;
  type: 'application' | 'invitation';
  status: RequestStatus;
  cancelReason: CancelReason | null;
  message: string | null;
  project: {
    id: string;
    name: string;
    status: ProjectStatus;
    category: NamedRef;
  };
  role: NamedRef;
  student: UserRef & { rating: Rating };
  invitedBy: PersonRef | null;
  createdAt: string;
  expiresAt: string;
  resolvedAt: string | null;
  viewer: RequestViewer;
};

export type MyRequests = {
  sent: RecruitmentRequest[];
  received: RecruitmentRequest[];
};

// Projects

export type ApplyBlock = { code: string; message: string };

export type ProjectViewer = {
  isLead: boolean;
  isMember: boolean;
  pendingRequest: RecruitmentRequest | null;
  applyBlock: ApplyBlock | null;
};

export type ProjectDetail = ProjectCard & {
  createdAt: string;
  startedAt: string | null;
  completedAt: string | null;
  cancelledAt: string | null;
  expectedEndDate: string | null;
  members: Member[];
  contact: { lineUrl: string | null; discordUrl: string | null } | null;
  pendingTransfer: {
    id: string;
    toUserId: string;
    toDisplayName: string;
    fromUserId: string;
  } | null;
  ratingWindow: { closesAt: string; open: boolean } | null;
  endDateReached: boolean;
  viewer: ProjectViewer;
};

export type MyProject = ProjectCard & {
  isLead: boolean;
  roleName: string;
  ratingsOpen: boolean;
  hasPendingTransferToMe: boolean;
};

export type ProjectInput = {
  name: string;
  description: string;
  categoryId: string;
  durationValue: number;
  durationUnit: DurationUnit;
};

export type NewProjectInput = ProjectInput & {
  roles: { name: string; capacity: number }[];
  leadRoleIndex: number;
};

// Accounts

export type Me = {
  id: string;
  email: string;
  displayName: string | null;
  profileComplete: boolean;
  unreadNotifications: number;
};

export type DemoAccount = { email: string; displayName: string; note: string };
export type Faculty = NamedRef;
export type Category = NamedRef;
export type Skill = { id: string; name: string; isPredefined: boolean };
export type ItemList<T> = { items: T[] };

// Profiles

export type AvailabilitySlot = { day: number; start: string };

export type ProfileCard = {
  userId: string;
  displayName: string;
  faculty: string | null;
  year: number | null;
  headline: string | null;
  skills: string[];
  rating: Rating;
};

export type InvitableProject = {
  id: string;
  name: string;
  roles: { id: string; name: string; available: number }[];
};

export type Profile = {
  userId: string;
  displayName: string;
  faculty: NamedRef | null;
  year: number | null;
  headline: string | null;
  workingStyle: string | null;
  skills: string[];
  availability: AvailabilitySlot[];
  rating: Rating;
  isSelf: boolean;
  cv: { available: boolean; fileName: string | null };
  viewerActions: { invitableProjects: InvitableProject[] };
};

export type MyProfile = Omit<Profile, 'displayName'> & {
  displayName: string | null;
  email: string;
  cvShared: boolean;
  searchable: boolean;
};

export type ProfileInput = {
  displayName: string | null;
  facultyId: string | null;
  year: number | null;
  headline: string | null;
  workingStyle: string | null;
  skills: string[];
};

export type CvFile = {
  fileName: string;
  sizeBytes: number;
  uploadedAt: string;
};

export type HistoryItem = {
  project: { id: string; name: string; status: ProjectStatus };
  roleName: string;
  joinedAt: string;
  leftAt: string | null;
  leftReason: string | null;
  endedAt: string | null;
};

export type ProfileHistory = { current: HistoryItem[]; past: HistoryItem[] };

export type ProfileRatings = {
  rating: Rating;
  published: { score: number; reviewer: PersonRef; project: NamedRef }[];
  pendingProjects: NamedRef[];
};

export type TeammateRating = {
  teammate: PersonRef;
  project: NamedRef;
  state: 'published' | 'pending' | 'not_rated';
  score: number | null;
};

// Notifications

export type Notification = {
  id: string;
  type: string;
  message: string;
  link: string;
  createdAt: string;
  readAt: string | null;
};

export type NotificationPage = Page<Notification> & { unreadCount: number };

export type ProjectRatings = {
  closesAt: string;
  open: boolean;
  eligible: boolean;
  teammates: { user: PersonRef; roleName: string; myScore: number | null }[];
  published: { reviewer: PersonRef; recipient: PersonRef; score: number }[];
  statusText: 'Ratings pending' | null;
};
