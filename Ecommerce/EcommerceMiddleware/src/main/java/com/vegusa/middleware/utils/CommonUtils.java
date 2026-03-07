package com.vegusa.middleware.utils;

import com.vegusa.middleware.entity.Company;
import com.vegusa.middleware.repository.local.CompanyRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;

@Component
public class CommonUtils {
    @Autowired
    private CompanyRepository companyRepository;

    public Company getCompany(String dataAreaId){
        return companyRepository.getCompany(dataAreaId);
    }

    public <T> List<List<T>> partitionList(List<T> list, int batchSize) {
        List<List<T>> partitions = new ArrayList<>();
        for (int i = 0; i < list.size(); i += batchSize) {
            partitions.add(list.subList(i, Math.min(i + batchSize, list.size())));
        }
        return partitions;
    }
}
