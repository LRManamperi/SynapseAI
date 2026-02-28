# Frontend Setup and Installation Guide

## Prerequisites

- Node.js 18+ installed
- npm or yarn package manager

## Installation Steps

### 1. Install Dependencies

Navigate to the frontend directory and install packages:

```powershell
cd frontend\web
npm install
```

This will install:
- Next.js 14.1.0
- React 18.2.0
- TypeScript 5
- Tailwind CSS 3.3.0
- Axios (API client)
- Zustand (State management)

### 2. Environment Configuration

The `.env.local` file is already configured with:

```env
NEXT_PUBLIC_API_URL=http://localhost:80
NODE_ENV=development
```

**Important:** Change `NEXT_PUBLIC_API_URL` to match your API Gateway URL in production.

### 3. Development Server

Start the Next.js development server:

```powershell
npm run dev
```

The application will be available at: **http://localhost:3000**

### 4. Build for Production

To create an optimized production build:

```powershell
npm run build
npm start
```

## Frontend Structure

```
frontend/web/
├── app/                    # Next.js App Router
│   ├── layout.tsx         # Root layout with metadata
│   ├── page.tsx           # Landing page
│   ├── globals.css        # Global styles + Tailwind
│   ├── login/
│   │   └── page.tsx       # Login page
│   ├── register/
│   │   └── page.tsx       # Registration page
│   ├── dashboard/
│   │   └── page.tsx       # User dashboard
│   └── upload/
│       └── page.tsx       # Content upload page
├── lib/
│   └── api.ts             # API client with Axios
├── store/
│   └── authStore.ts       # Zustand auth store
├── package.json           # Dependencies
├── tsconfig.json          # TypeScript config
├── tailwind.config.js     # Tailwind configuration
├── postcss.config.js      # PostCSS config
├── next.config.js         # Next.js config
└── .env.local            # Environment variables
```

## Features

### Authentication
- ✅ User registration with validation
- ✅ Login with JWT tokens
- ✅ Token refresh mechanism
- ✅ Auto-redirect on 401 errors
- ✅ Persistent auth state (localStorage + Zustand)

### Content Management
- ✅ File upload (PDF, DOC, DOCX, TXT)
- ✅ Content listing with pagination
- ✅ Auto-generated AI quizzes

### UI/UX
- ✅ Responsive design (mobile-first)
- ✅ Dark mode support
- ✅ Loading states
- ✅ Error handling
- ✅ Form validation
- ✅ Tailwind CSS utility classes

## API Integration

The frontend communicates with the backend through the API Gateway (Nginx) at port 80.

### API Endpoints Used

**Auth Service:**
- `POST /api/auth/register` - User registration
- `POST /api/auth/login` - User login
- `POST /api/auth/refresh` - Token refresh

**Content Service:**
- `POST /api/content/upload` - Upload content
- `GET /api/content/list` - List user's content
- `GET /api/content/:id` - Get specific content
- `DELETE /api/content/:id` - Delete content

**Quiz Service:**
- `GET /api/quiz/:id` - Get quiz details
- `POST /api/quiz/:id/submit` - Submit quiz attempt
- `GET /api/quiz/list` - List quizzes
- `GET /api/quiz/:id/history` - Get attempt history

## State Management

### Zustand Auth Store

Located in `store/authStore.ts`:

```typescript
interface AuthState {
  user: User | null
  accessToken: string | null
  refreshToken: string | null
  setAuth: (user, accessToken, refreshToken) => void
  clearAuth: () => void
  isAuthenticated: () => boolean
}
```

**Usage:**
```typescript
const { user, setAuth, clearAuth, isAuthenticated } = useAuthStore()
```

## Styling

### Tailwind CSS Custom Classes

Defined in `app/globals.css`:

- `.btn-primary` - Primary button style
- `.btn-secondary` - Secondary button style
- `.input` - Form input style
- `.card` - Card container style

### Custom Colors

Primary color palette (blue):
- `primary-50` to `primary-900`

### Dark Mode

Dark mode uses Tailwind's `dark:` prefix and is configured with `class` strategy in `tailwind.config.js`.

## Troubleshooting

### Module Not Found Errors

If you see "Cannot find module 'next'" errors:

```powershell
cd frontend\web
npm install
```

### Build Errors

Clear Next.js cache:

```powershell
rm -rf .next
npm run build
```

### TypeScript Errors

Regenerate TypeScript types:

```powershell
rm next-env.d.ts
npm run dev  # This will regenerate it
```

### API Connection Issues

1. Verify backend services are running
2. Check `.env.local` has correct `NEXT_PUBLIC_API_URL`
3. Ensure Nginx gateway is running on port 80
4. Check browser console for CORS errors

### CORS Issues

If you see CORS errors, ensure Nginx is configured with CORS headers (already configured in `gateway/nginx/nginx.conf`).

## Development Tips

### Hot Reload

Next.js automatically reloads when you save files. If it doesn't:

```powershell
# Restart dev server
npm run dev
```

### Debugging

Use React DevTools extension in Chrome/Firefox for component inspection.

Check Network tab in browser DevTools to see API calls.

### Performance

For production builds, Next.js automatically:
- Minifies JavaScript
- Optimizes images
- Generates static pages where possible
- Code splits by route

## Next Steps

After frontend is running:

1. Test user registration flow
2. Upload sample content
3. Check dashboard statistics
4. Verify quiz generation works
5. Test progress tracking

## Support

For issues, check:
- [Next.js Documentation](https://nextjs.org/docs)
- [Tailwind CSS Documentation](https://tailwindcss.com/docs)
- [TypeScript Handbook](https://www.typescriptlang.org/docs/)
