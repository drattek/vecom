package com.vegusa.middleware.utils;

import com.vegusa.middleware.entity.Company;
import com.vegusa.middleware.repository.CompanyRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

@Component
public class CommonUtils {
    @Autowired
    private CompanyRepository companyRepository;

    public Company getCompany(String dataAreaId){
        return companyRepository.getCompany(dataAreaId);
    }
}
