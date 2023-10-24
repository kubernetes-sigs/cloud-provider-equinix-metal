/*
Load Balancer Management API

Load Balancer Management API is an API for managing load balancers.

API version: 0.0.1
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package v1

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PoolsApiService PoolsApi service
type PoolsApiService service

type ApiCreateLoadBalancerPoolOriginRequest struct {
	ctx                          context.Context
	ApiService                   *PoolsApiService
	loadBalancerPoolID           string
	loadBalancerPoolOriginCreate *LoadBalancerPoolOriginCreate
}

func (r ApiCreateLoadBalancerPoolOriginRequest) LoadBalancerPoolOriginCreate(loadBalancerPoolOriginCreate LoadBalancerPoolOriginCreate) ApiCreateLoadBalancerPoolOriginRequest {
	r.loadBalancerPoolOriginCreate = &loadBalancerPoolOriginCreate
	return r
}

func (r ApiCreateLoadBalancerPoolOriginRequest) Execute() (*ResourceCreatedResponse, *http.Response, error) {
	return r.ApiService.CreateLoadBalancerPoolOriginExecute(r)
}

/*
CreateLoadBalancerPoolOrigin Create a load balancer origin for a pool.

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param loadBalancerPoolID ID of the load balancer pool to get
	@return ApiCreateLoadBalancerPoolOriginRequest
*/
func (a *PoolsApiService) CreateLoadBalancerPoolOrigin(ctx context.Context, loadBalancerPoolID string) ApiCreateLoadBalancerPoolOriginRequest {
	return ApiCreateLoadBalancerPoolOriginRequest{
		ApiService:         a,
		ctx:                ctx,
		loadBalancerPoolID: loadBalancerPoolID,
	}
}

// Execute executes the request
//
//	@return ResourceCreatedResponse
func (a *PoolsApiService) CreateLoadBalancerPoolOriginExecute(r ApiCreateLoadBalancerPoolOriginRequest) (*ResourceCreatedResponse, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPost
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *ResourceCreatedResponse
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PoolsApiService.CreateLoadBalancerPoolOrigin")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/loadbalancers/pools/{loadBalancerPoolID}/origins"
	localVarPath = strings.Replace(localVarPath, "{"+"loadBalancerPoolID"+"}", url.PathEscape(parameterValueToString(r.loadBalancerPoolID, "loadBalancerPoolID")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.loadBalancerPoolOriginCreate == nil {
		return localVarReturnValue, nil, reportError("loadBalancerPoolOriginCreate is required and must be specified")
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	// body params
	localVarPostBody = r.loadBalancerPoolOriginCreate
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiDeleteLoadBalancerPoolRequest struct {
	ctx                context.Context
	ApiService         *PoolsApiService
	loadBalancerPoolID string
}

func (r ApiDeleteLoadBalancerPoolRequest) Execute() (*http.Response, error) {
	return r.ApiService.DeleteLoadBalancerPoolExecute(r)
}

/*
DeleteLoadBalancerPool Delete a load balancer pool.

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param loadBalancerPoolID ID of the load balancer pool to get
	@return ApiDeleteLoadBalancerPoolRequest
*/
func (a *PoolsApiService) DeleteLoadBalancerPool(ctx context.Context, loadBalancerPoolID string) ApiDeleteLoadBalancerPoolRequest {
	return ApiDeleteLoadBalancerPoolRequest{
		ApiService:         a,
		ctx:                ctx,
		loadBalancerPoolID: loadBalancerPoolID,
	}
}

// Execute executes the request
func (a *PoolsApiService) DeleteLoadBalancerPoolExecute(r ApiDeleteLoadBalancerPoolRequest) (*http.Response, error) {
	var (
		localVarHTTPMethod = http.MethodDelete
		localVarPostBody   interface{}
		formFiles          []formFile
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PoolsApiService.DeleteLoadBalancerPool")
	if err != nil {
		return nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/loadbalancers/pools/{loadBalancerPoolID}"
	localVarPath = strings.Replace(localVarPath, "{"+"loadBalancerPoolID"+"}", url.PathEscape(parameterValueToString(r.loadBalancerPoolID, "loadBalancerPoolID")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarHTTPResponse, newErr
	}

	return localVarHTTPResponse, nil
}

type ApiGetLoadBalancerPoolRequest struct {
	ctx                context.Context
	ApiService         *PoolsApiService
	loadBalancerPoolID string
}

func (r ApiGetLoadBalancerPoolRequest) Execute() (*LoadBalancerPool, *http.Response, error) {
	return r.ApiService.GetLoadBalancerPoolExecute(r)
}

/*
GetLoadBalancerPool Gets a load balancer pool by ID

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param loadBalancerPoolID ID of the load balancer pool to get
	@return ApiGetLoadBalancerPoolRequest
*/
func (a *PoolsApiService) GetLoadBalancerPool(ctx context.Context, loadBalancerPoolID string) ApiGetLoadBalancerPoolRequest {
	return ApiGetLoadBalancerPoolRequest{
		ApiService:         a,
		ctx:                ctx,
		loadBalancerPoolID: loadBalancerPoolID,
	}
}

// Execute executes the request
//
//	@return LoadBalancerPool
func (a *PoolsApiService) GetLoadBalancerPoolExecute(r ApiGetLoadBalancerPoolRequest) (*LoadBalancerPool, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *LoadBalancerPool
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PoolsApiService.GetLoadBalancerPool")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/loadbalancers/pools/{loadBalancerPoolID}"
	localVarPath = strings.Replace(localVarPath, "{"+"loadBalancerPoolID"+"}", url.PathEscape(parameterValueToString(r.loadBalancerPoolID, "loadBalancerPoolID")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiListLoadBalancerPoolOriginsRequest struct {
	ctx                context.Context
	ApiService         *PoolsApiService
	loadBalancerPoolID string
}

func (r ApiListLoadBalancerPoolOriginsRequest) Execute() (*LoadBalancerPoolOriginCollection, *http.Response, error) {
	return r.ApiService.ListLoadBalancerPoolOriginsExecute(r)
}

/*
ListLoadBalancerPoolOrigins Gets the origins for a pool.

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param loadBalancerPoolID ID of the load balancer pool to get
	@return ApiListLoadBalancerPoolOriginsRequest
*/
func (a *PoolsApiService) ListLoadBalancerPoolOrigins(ctx context.Context, loadBalancerPoolID string) ApiListLoadBalancerPoolOriginsRequest {
	return ApiListLoadBalancerPoolOriginsRequest{
		ApiService:         a,
		ctx:                ctx,
		loadBalancerPoolID: loadBalancerPoolID,
	}
}

// Execute executes the request
//
//	@return LoadBalancerPoolOriginCollection
func (a *PoolsApiService) ListLoadBalancerPoolOriginsExecute(r ApiListLoadBalancerPoolOriginsRequest) (*LoadBalancerPoolOriginCollection, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodGet
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *LoadBalancerPoolOriginCollection
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PoolsApiService.ListLoadBalancerPoolOrigins")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/loadbalancers/pools/{loadBalancerPoolID}/origins"
	localVarPath = strings.Replace(localVarPath, "{"+"loadBalancerPoolID"+"}", url.PathEscape(parameterValueToString(r.loadBalancerPoolID, "loadBalancerPoolID")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiUpdateLoadBalancerPoolRequest struct {
	ctx                    context.Context
	ApiService             *PoolsApiService
	loadBalancerPoolID     string
	loadBalancerPoolUpdate *LoadBalancerPoolUpdate
}

func (r ApiUpdateLoadBalancerPoolRequest) LoadBalancerPoolUpdate(loadBalancerPoolUpdate LoadBalancerPoolUpdate) ApiUpdateLoadBalancerPoolRequest {
	r.loadBalancerPoolUpdate = &loadBalancerPoolUpdate
	return r
}

func (r ApiUpdateLoadBalancerPoolRequest) Execute() (*LoadBalancerPool, *http.Response, error) {
	return r.ApiService.UpdateLoadBalancerPoolExecute(r)
}

/*
UpdateLoadBalancerPool Update a load balancer pool.

	@param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
	@param loadBalancerPoolID ID of the load balancer pool to get
	@return ApiUpdateLoadBalancerPoolRequest
*/
func (a *PoolsApiService) UpdateLoadBalancerPool(ctx context.Context, loadBalancerPoolID string) ApiUpdateLoadBalancerPoolRequest {
	return ApiUpdateLoadBalancerPoolRequest{
		ApiService:         a,
		ctx:                ctx,
		loadBalancerPoolID: loadBalancerPoolID,
	}
}

// Execute executes the request
//
//	@return LoadBalancerPool
func (a *PoolsApiService) UpdateLoadBalancerPoolExecute(r ApiUpdateLoadBalancerPoolRequest) (*LoadBalancerPool, *http.Response, error) {
	var (
		localVarHTTPMethod  = http.MethodPatch
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *LoadBalancerPool
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "PoolsApiService.UpdateLoadBalancerPool")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1/loadbalancers/pools/{loadBalancerPoolID}"
	localVarPath = strings.Replace(localVarPath, "{"+"loadBalancerPoolID"+"}", url.PathEscape(parameterValueToString(r.loadBalancerPoolID, "loadBalancerPoolID")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.loadBalancerPoolUpdate == nil {
		return localVarReturnValue, nil, reportError("loadBalancerPoolUpdate is required and must be specified")
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	// body params
	localVarPostBody = r.loadBalancerPoolUpdate
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}