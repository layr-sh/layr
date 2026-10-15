import type { Configuration } from '../../generated/runtime';
import {
  AnonymousApi,
  ConfigApi,
  MFAApi,
  OIDCApi,
  OTPApi,
  PasskeysSignInApi,
  PasskeysSignUpApi,
  PasswordResetApi,
  SignInApi,
  SignOutApi,
  SignUpApi,
  TokenApi,
  UserApi,
  UserEmailApi,
  UserEmailVerificationApi,
  UserPasskeysApi,
  UserPasswordApi,
  UserPhoneApi,
  UserPhoneVerificationApi,
  UserPropertiesApi,
  UserSessionsApi,
  UsersApi,
  UsersSessionsApi,
  type AuthUsersListRequest,
} from '../../generated/apis';
import type {
  AuthTokenResponse,
  BeginPasskeySignUpInput,
  ChallengeMFAInput,
  Config,
  ConfirmEmailVerificationInput,
  ConfirmPasswordResetInput,
  ConfirmPhoneVerificationInput,
  CreateUserInput,
  ExportUserResponse,
  GetOIDCUserInfoResponse,
  GetUserResponse,
  IssueOIDCTokenResponse,
  ListSessionsResponse,
  ListUserSessionsResponse,
  ListUsersResponse,
  LockUserInput,
  Passkey,
  RefreshTokenInput,
  RequestEmailVerificationInput,
  RequestPasswordResetInput,
  RequestPhoneVerificationInput,
  RevokeOtherSessionsResponse,
  SendOTPInput,
  SetupMFAResponse,
  SignInAnonymousInput,
  SignInInput,
  SignInOptions,
  SignUpInput,
  SignUpOptions,
  UpdateUserEmailInput,
  UpdateUserPasswordInput,
  UpdateUserPhoneInput,
  UpdateUserPropertiesInput,
  UpdateUserPropertiesResponse,
  User,
  VerifyMFAInput,
  VerifyOTPInput,
  VerifyPasskeySignInInput,
  VerifyPasskeySignUpInput,
} from '../../generated/models';
import { ATTACH_CONTROL_PLANE, type ElevatableService, type LayrClient } from '../core/client';
import { handleApiCall } from '../core/transport';

export interface AuthControlPlane {
  config: {
    get: () => Promise<Config>;
    update: (input: Config) => Promise<Config>;
  };
  users: {
    create: (input: CreateUserInput) => Promise<User>;
    delete: (userId: string) => Promise<void>;
    get: (userId: string) => Promise<User>;
    list: (params?: AuthUsersListRequest) => Promise<ListUsersResponse>;
    lock: (userId: string, input: LockUserInput) => Promise<User>;
    unlock: (userId: string) => Promise<User>;
    sessions: {
      list: (userId: string) => Promise<ListUserSessionsResponse>;
      revoke: (userId: string) => Promise<void>;
    };
  };
}

export interface AuthService extends ElevatableService<AuthControlPlane> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: Configuration) => AuthControlPlane;

  sign_in: (input: SignInInput) => Promise<AuthTokenResponse>;
  sign_out: () => Promise<void>;
  sign_up: (input: SignUpInput) => Promise<AuthTokenResponse>;

  anonymous: {
    sign_in: (input?: SignInAnonymousInput) => Promise<AuthTokenResponse>;
  };

  mfa: {
    challenge: (input: ChallengeMFAInput) => Promise<AuthTokenResponse>;
    disable: () => Promise<void>;
    setup: () => Promise<SetupMFAResponse>;
    verify: (input: VerifyMFAInput) => Promise<AuthTokenResponse>;
  };

  oidc: {
    token: (body?: object) => Promise<IssueOIDCTokenResponse>;
    userinfo: () => Promise<GetOIDCUserInfoResponse>;
  };

  otp: {
    send: (input: SendOTPInput) => Promise<void>;
    verify: (input: VerifyOTPInput) => Promise<AuthTokenResponse>;
  };

  passkeys: {
    sign_in: {
      begin: () => Promise<SignInOptions>;
      verify: (input: VerifyPasskeySignInInput) => Promise<AuthTokenResponse>;
    };
    sign_up: {
      begin: (input: BeginPasskeySignUpInput) => Promise<SignUpOptions>;
      verify: (input: VerifyPasskeySignUpInput) => Promise<AuthTokenResponse>;
    };
  };

  password_reset: {
    confirm: (input: ConfirmPasswordResetInput) => Promise<void>;
    request: (input: RequestPasswordResetInput) => Promise<void>;
  };

  token: {
    refresh: (input: RefreshTokenInput) => Promise<AuthTokenResponse>;
  };

  user: {
    delete: () => Promise<void>;
    export: () => Promise<ExportUserResponse>;
    get: () => Promise<GetUserResponse>;
    email: {
      update: (input: UpdateUserEmailInput) => Promise<void>;
      verification: {
        confirm: (input: ConfirmEmailVerificationInput) => Promise<void>;
        request: (input?: RequestEmailVerificationInput) => Promise<void>;
      };
    };
    passkeys: {
      delete: (passkeyId: string) => Promise<void>;
      list: () => Promise<Passkey[]>;
    };
    password: {
      update: (input: UpdateUserPasswordInput) => Promise<void>;
    };
    phone: {
      update: (input: UpdateUserPhoneInput) => Promise<void>;
      verification: {
        confirm: (input: ConfirmPhoneVerificationInput) => Promise<void>;
        request: (input?: RequestPhoneVerificationInput) => Promise<void>;
      };
    };
    properties: {
      update: (input: UpdateUserPropertiesInput) => Promise<UpdateUserPropertiesResponse>;
    };
    sessions: {
      list: () => Promise<ListSessionsResponse>;
      revoke: (sessionId: string) => Promise<void>;
      revoke_others: () => Promise<RevokeOtherSessionsResponse>;
    };
  };
}

export function useAuth(client: LayrClient): AuthService {
  const config = client.config;

  const service: AuthService = {
    client,

    sign_in: (input: SignInInput) =>
      handleApiCall(() => new SignInApi(config).authSignIn({ signInInput: input })),
    sign_out: () =>
      handleApiCall(async () => {
        await new SignOutApi(config).authSignOut({ body: null });
      }),
    sign_up: (input: SignUpInput) =>
      handleApiCall(() => new SignUpApi(config).authSignUp({ signUpInput: input })),

    anonymous: {
      sign_in: (input: SignInAnonymousInput = {}) =>
        handleApiCall(() =>
          new AnonymousApi(config).authAnonymousSignIn({ signInAnonymousInput: input })
        ),
    },

    mfa: {
      challenge: (input: ChallengeMFAInput) =>
        handleApiCall(() => new MFAApi(config).authMfaChallenge({ challengeMFAInput: input })),
      disable: () => handleApiCall(() => new MFAApi(config).authMfaDisable()),
      setup: () => handleApiCall(() => new MFAApi(config).authMfaSetup({ body: {} })),
      verify: (input: VerifyMFAInput) =>
        handleApiCall(() => new MFAApi(config).authMfaVerify({ verifyMFAInput: input })),
    },

    oidc: {
      token: (body: object = {}) =>
        handleApiCall(() => new OIDCApi(config).authOidcToken({ body })),
      userinfo: () => handleApiCall(() => new OIDCApi(config).authOidcUserinfo()),
    },

    otp: {
      send: (input: SendOTPInput) =>
        handleApiCall(() => new OTPApi(config).authOtpSend({ sendOTPInput: input })),
      verify: (input: VerifyOTPInput) =>
        handleApiCall(() => new OTPApi(config).authOtpVerify({ verifyOTPInput: input })),
    },

    passkeys: {
      sign_in: {
        begin: () =>
          handleApiCall(() =>
            new PasskeysSignInApi(config).authPasskeysSignInBegin({ body: {} })
          ),
        verify: (input: VerifyPasskeySignInInput) =>
          handleApiCall(() =>
            new PasskeysSignInApi(config).authPasskeysSignInVerify({
              verifyPasskeySignInInput: input,
            })
          ),
      },
      sign_up: {
        begin: (input: BeginPasskeySignUpInput) =>
          handleApiCall(() =>
            new PasskeysSignUpApi(config).authPasskeysSignUpBegin({
              beginPasskeySignUpInput: input,
            })
          ),
        verify: (input: VerifyPasskeySignUpInput) =>
          handleApiCall(() =>
            new PasskeysSignUpApi(config).authPasskeysSignUpVerify({
              verifyPasskeySignUpInput: input,
            })
          ),
      },
    },

    password_reset: {
      confirm: (input: ConfirmPasswordResetInput) =>
        handleApiCall(() =>
          new PasswordResetApi(config).authPasswordResetConfirm({
            confirmPasswordResetInput: input,
          })
        ),
      request: (input: RequestPasswordResetInput) =>
        handleApiCall(() =>
          new PasswordResetApi(config).authPasswordResetRequest({
            requestPasswordResetInput: input,
          })
        ),
    },

    token: {
      refresh: (input: RefreshTokenInput) =>
        handleApiCall(() => new TokenApi(config).authTokenRefresh({ refreshTokenInput: input })),
    },

    user: {
      delete: () => handleApiCall(() => new UserApi(config).authUserDelete()),
      export: () =>
        handleApiCall(() => new UserApi(config).authUserExport({ body: null })),
      get: () => handleApiCall(() => new UserApi(config).authUserGet()),
      email: {
        update: (input: UpdateUserEmailInput) =>
          handleApiCall(() =>
            new UserEmailApi(config).authUserEmailUpdate({ updateUserEmailInput: input })
          ),
        verification: {
          confirm: (input: ConfirmEmailVerificationInput) =>
            handleApiCall(() =>
              new UserEmailVerificationApi(config).authUserEmailVerificationConfirm({
                confirmEmailVerificationInput: input,
              })
            ),
          request: (input: RequestEmailVerificationInput = {}) =>
            handleApiCall(() =>
              new UserEmailVerificationApi(config).authUserEmailVerificationRequest({
                requestEmailVerificationInput: input,
              })
            ),
        },
      },
      passkeys: {
        delete: (passkeyId: string) =>
          handleApiCall(() => new UserPasskeysApi(config).authUserPasskeysDelete({ passkeyId })),
        list: () => handleApiCall(() => new UserPasskeysApi(config).authUserPasskeysList()),
      },
      password: {
        update: (input: UpdateUserPasswordInput) =>
          handleApiCall(() =>
            new UserPasswordApi(config).authUserPasswordUpdate({
              updateUserPasswordInput: input,
            })
          ),
      },
      phone: {
        update: (input: UpdateUserPhoneInput) =>
          handleApiCall(() =>
            new UserPhoneApi(config).authUserPhoneUpdate({ updateUserPhoneInput: input })
          ),
        verification: {
          confirm: (input: ConfirmPhoneVerificationInput) =>
            handleApiCall(() =>
              new UserPhoneVerificationApi(config).authUserPhoneVerificationConfirm({
                confirmPhoneVerificationInput: input,
              })
            ),
          request: (input: RequestPhoneVerificationInput = {}) =>
            handleApiCall(() =>
              new UserPhoneVerificationApi(config).authUserPhoneVerificationRequest({
                requestPhoneVerificationInput: input,
              })
            ),
        },
      },
      properties: {
        update: (input: UpdateUserPropertiesInput) =>
          handleApiCall(() =>
            new UserPropertiesApi(config).authUserPropertiesUpdate({
              updateUserPropertiesInput: input,
            })
          ),
      },
      sessions: {
        list: () => handleApiCall(() => new UserSessionsApi(config).authUserSessionsList()),
        revoke: (sessionId: string) =>
          handleApiCall(() => new UserSessionsApi(config).authUserSessionsRevoke({ sessionId })),
        revoke_others: () =>
          handleApiCall(() => new UserSessionsApi(config).authUserSessionsRevokeOthers({ body: null })),
      },
    },

    [ATTACH_CONTROL_PLANE]: (elevatedConfig: Configuration) => ({
      config: {
        get: () => handleApiCall(() => new ConfigApi(elevatedConfig).authConfigGet()),
        update: (input: Config) =>
          handleApiCall(() =>
            new ConfigApi(elevatedConfig).authConfigUpdate({ config: input })
          ),
      },
      users: {
        create: (input: CreateUserInput) =>
          handleApiCall(() =>
            new UsersApi(elevatedConfig).authUsersCreate({ createUserInput: input })
          ),
        delete: (userId: string) =>
          handleApiCall(() => new UsersApi(elevatedConfig).authUsersDelete({ userId })),
        get: (userId: string) =>
          handleApiCall(() => new UsersApi(elevatedConfig).authUsersGet({ userId })),
        list: (params: AuthUsersListRequest = {}) =>
          handleApiCall(() => new UsersApi(elevatedConfig).authUsersList(params)),
        lock: (userId: string, input: LockUserInput) =>
          handleApiCall(() =>
            new UsersApi(elevatedConfig).authUsersLock({
              userId,
              lockUserInput: input,
            })
          ),
        unlock: (userId: string) =>
          handleApiCall(() => new UsersApi(elevatedConfig).authUsersUnlock({ userId })),
        sessions: {
          list: (userId: string) =>
            handleApiCall(() =>
              new UsersSessionsApi(elevatedConfig).authUsersSessionsList({ userId })
            ),
          revoke: (userId: string) =>
            handleApiCall(() =>
              new UsersSessionsApi(elevatedConfig).authUsersSessionsRevoke({ userId, body: {} })
            ),
        },
      },
    }),
  };

  return service;
}
